/* A tool to export your mealie recipes for offline storage.
Copyright (C) 2025  Torsten Long

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/webp"
)

func collapseWhitespace(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// We only define those fields that we actually want to use.
type recipe struct {
	ID           string        `json:"id"`
	Slug         string        `json:"slug"`
	Name         string        `json:"name"`
	Servings     float32       `json:"recipeServings"`
	TotalTime    string        `json:"totalTime"`
	Description  string        `json:"description"`
	OrgURL       string        `json:"orgURL"`
	Categories   []organiser   `json:"recipeCategory"`
	Tags         []organiser   `json:"tags"`
	Instructions []instruction `json:"recipeInstructions"`
	Ingredients  []ingredient  `json:"recipeIngredient"`
	Comments     []comment     `json:"comments"`
	Image        string        `json:"image"`
}

func (r *recipe) normalise() {
	r.ID = collapseWhitespace(r.ID)
	r.Name = collapseWhitespace(r.Name)
	r.TotalTime = collapseWhitespace(r.TotalTime)
	r.Description = collapseWhitespace(r.Description)
	r.OrgURL = collapseWhitespace(r.OrgURL)
	r.Image = collapseWhitespace(r.Image)
	for _, category := range r.Categories {
		category.normalise()
	}
	for _, tag := range r.Tags {
		tag.normalise()
	}
	for _, instruction := range r.Instructions {
		instruction.normalise()
	}
	for _, ingredient := range r.Ingredients {
		ingredient.normalise()
	}
	for _, comment := range r.Comments {
		comment.normalise()
	}
}

type instruction struct {
	Text string `json:"text"`
}

func (i *instruction) normalise() {
	i.Text = collapseWhitespace(i.Text)
}

type ingredient struct {
	Text string `json:"display"`
}

func (i *ingredient) normalise() {
	i.Text = collapseWhitespace(i.Text)
}

type organiser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (o *organiser) normalise() {
	o.Name = collapseWhitespace(o.Name)
}

type comment struct {
	Text string `json:"text"`
	User user   `json:"user"`
}

func (c *comment) normalise() {
	c.Text = collapseWhitespace(c.Text)
	c.User.normalise()
}

type user struct {
	Name string `json:"username"`
}

func (u *user) normalise() {
	u.Name = collapseWhitespace(u.Name)
}

type slugsResponse struct {
	Items []slug `json:"items"`
	Pages int    `json:"total_pages"`
}

type userResponse struct {
	Name      string `json:"username"`
	Group     string `json:"group"`
	Household string `json:"household"`
}

func (u userResponse) String() string {
	return fmt.Sprintf("%s (group=%s, household=%s)", u.Name, u.Group, u.Household)
}

type slug struct {
	Slug string `json:"slug"`
}

type (
	getRecipesFn func(ctx context.Context, queryParams map[string][]string) ([]recipe, error)
	getMediaFn   func(ctx context.Context, uuid, filename, middle string) (mediaDownload, error)
)

type mealie struct {
	url     string
	token   string
	limiter chan bool
	// defaultQuery map[string][]string
}

func (m *mealie) getSlugs(ctx context.Context, query *url.Values) ([]slug, error) {
	log.Println("getting slugs")

	if query == nil {
		query = &url.Values{}
	}

	page := 1
	lastPage := 10
	var slugs []slug

	for page <= lastPage {
		query.Set("page", fmt.Sprint(page))
		query.Set("perPage", "200")

		var slugsResponse slugsResponse

		req, err := http.NewRequestWithContext(ctx, "GET", m.url+"/api/recipes", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = query.Encode()
		log.Println("getting from", m.url+"/api/recipes?"+req.URL.RawQuery)

		m.addAuth(req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		}
		err = json.Unmarshal(body, &slugsResponse)
		if err != nil {
			log.Println("body", string(body))
			return nil, err
		}
		lastPage = slugsResponse.Pages
		slugs = append(slugs, slugsResponse.Items...)
		log.Printf("retrieved %d slugs from page %d", len(slugsResponse.Items), page)

		page++
	}

	log.Printf("retrieved %d slugs in total", len(slugs))
	return slugs, nil
}

func (m *mealie) getRecipe(ctx context.Context, slug string) (recipe, error) {
	var recipe recipe
	req, err := http.NewRequestWithContext(ctx, "GET", m.url+"/api/recipes/"+slug, nil)
	if err != nil {
		return recipe, err
	}
	log.Println("getting from", m.url+"/api/recipes/"+slug)
	m.addAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return recipe, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return recipe, err
	}
	if resp.StatusCode != http.StatusOK {
		return recipe, fmt.Errorf(
			"slug %s: unexpected status code %d: %s", slug, resp.StatusCode, string(body),
		)
	}
	err = json.Unmarshal(body, &recipe)
	if err != nil {
		log.Println("body", string(body))
		return recipe, err
	}
	return recipe, err
}

func (m mealie) getRecipes(ctx context.Context, queryParams map[string][]string) ([]recipe, error) {
	log.Println("retrieving recipes")

	// Build the raw query string for later use.
	query := url.Values{}
	for key, values := range queryParams {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	log.Println("built query string", &query)

	// First, we retrieve the recipe slugs. We start with page 1 and then use the "next" link to
	// paginate.
	slugs, err := m.getSlugs(ctx, &query)
	if err != nil {
		return nil, err
	}

	// Then, we retrieve the information about all the recipes. We send many requests in parallel to
	// speed up the process.
	wg := sync.WaitGroup{}
	wg.Add(len(slugs))
	recipes := make([]recipe, len(slugs))
	errs := make([]error, len(slugs))

	for idx, slug := range slugs {
		// Avoid loop pointer weirdness.
		id := idx
		slug := slug
		// Retrieve all recipes in parallel. Let'ssee if this works.
		go func() {
			if m.limiter != nil {
				m.limiter <- true
			}
			recipe, err := m.getRecipe(ctx, slug.Slug)
			if err == nil {
				recipe.normalise()
				recipes[id] = recipe
			} else {
				errs[id] = err
			}
			wg.Done()
			if m.limiter != nil {
				<-m.limiter
			}
		}()
	}
	wg.Wait()

	return recipes, errors.Join(errs...)
}

type mediaDownload struct {
	content []byte
	mime    string
}

func (m mealie) getMedia(
	ctx context.Context,
	uuid string,
	filename string,
	middle string,
) (mediaDownload, error) {
	log.Printf("retrieving media %s/%s", uuid, filename)

	var extension string
	filenameParts := strings.Split(filename, ".")
	if len(filenameParts) >= 1 {
		extension = strings.ToLower(filenameParts[len(filenameParts)-1])
	}

	url := fmt.Sprintf("%s/api/media/recipes/%s/%s/%s", m.url, uuid, middle, filename)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return mediaDownload{}, err
	}
	req.Header.Set("Accept", "image/*")

	m.addAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return mediaDownload{}, err
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return mediaDownload{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return mediaDownload{}, fmt.Errorf(
			"unexpected status code %d: %s", resp.StatusCode, string(content),
		)
	}
	err = resp.Body.Close()
	if err != nil {
		return mediaDownload{}, err
	}

	data := mediaDownload{
		content: content,
		mime:    resp.Header.Get("Content-Type"),
	}
	var decodeErr error
	if !strings.HasPrefix(data.mime, "image/") {
		log.Println("mealie claims we received no image but we requested one, checking")
		switch extension {
		case "jpg":
			_, decodeErr = jpeg.Decode(bytes.NewReader(data.content))
			extension = "jpeg"
		case "jpeg":
			_, decodeErr = jpeg.Decode(bytes.NewReader(data.content))
		case "webp":
			_, decodeErr = webp.Decode(bytes.NewReader(data.content))
		}
	}
	data.mime = "image/" + extension
	if decodeErr != nil {
		return data, fmt.Errorf("failed to verify download as %s", data.mime)
	}

	log.Printf("successfully retrieved media: %s", data.mime)
	return data, nil
}

func (m mealie) reuploadImage(
	ctx context.Context,
	slug string,
) (bool, error) {
	recipe, err := m.getRecipe(ctx, slug)
	if err != nil {
		return false, err
	}
	if recipe.Image != "" {
		log.Printf("skipping reupload of image for %s", slug)
		// In this case, the recipe does have an image assigned to it. No reupload is needed, then.
		return false, nil
	}
	log.Printf("attempting reupload of image for %s", slug)

	// Download image first.
	url := fmt.Sprintf("%s/api/media/recipes/%s/images/original.webp", m.url, recipe.ID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "image/*")
	m.addAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	imageContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		// In this case, the recipe has an image assigned even though the "image" property is null.
		log.Printf("found image for %s", slug)
	case http.StatusNotFound:
		// In this case, the recipe really does not have an image assigned to it.
		log.Printf("there is no image for %s", slug)
		return false, nil
	default:
		return false, fmt.Errorf(
			"unexpected status code %d: %s", resp.StatusCode, string(imageContent),
		)
	}
	err = resp.Body.Close()
	if err != nil {
		return false, err
	}
	log.Printf("retrieved image for %s", slug)

	// Upload the image again using multipart/form-data.
	// Prepare multipart/form-data input.
	var uploadBuffer bytes.Buffer
	multipartWriter := multipart.NewWriter(&uploadBuffer)
	// Add the image file.
	imageWriter, err := multipartWriter.CreateFormFile("image", "original.webp")
	if err != nil {
		return false, err
	}
	_, err = io.Copy(imageWriter, bytes.NewReader(imageContent))
	if err != nil {
		return false, err
	}
	extensionWriter, err := multipartWriter.CreateFormField("extension")
	if err != nil {
		return false, err
	}
	_, err = io.Copy(extensionWriter, strings.NewReader("webp"))
	if err != nil {
		return false, err
	}
	// Close the multipart writer. Otherwise, the sent body would be incomplete.
	err = multipartWriter.Close()
	if err != nil {
		return false, err
	}

	url = fmt.Sprintf("%s/api/recipes/%s/image", m.url, slug)
	req, err = http.NewRequestWithContext(ctx, "PUT", url, &uploadBuffer)
	if err != nil {
		return false, err
	}
	// The content type header will also contain the multipart boundary.
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	m.addAuth(req)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf(
			"unexpected status code %d: %s", resp.StatusCode, string(body),
		)
	}
	log.Printf("reuploaded image for %s", slug)

	return true, nil
}

func (m mealie) addAuth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.token))
}

func (m mealie) check() (group string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) //nolint:mnd
	defer cancel()

	// Augment error no matter which one we get.
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to verify connection to mealie: %s", err.Error())
		}
	}()

	var user userResponse
	req, err := http.NewRequestWithContext(ctx, "GET", m.url+"/api/users/self", nil)
	if err != nil {
		return "", err
	}
	m.addAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
	err = json.Unmarshal(body, &user)
	if err != nil {
		return "", err
	}

	log.Println("successful login with user", user)
	return strings.ToLower(user.Group), nil
}

type organisersResponse struct {
	Items []organiser `json:"items"`
	Pages int         `json:"total_pages"`
}

func (m *mealie) getOrganisers(ctx context.Context, kind string) ([]organiser, error) {
	if kind != "categories" && kind != "tags" {
		return nil, fmt.Errorf("can only get categories or tags for now but not '%s'", kind)
	}
	log.Printf("getting %s", kind)

	page := 1
	lastPage := 10
	var slugs []organiser
	query := url.Values{}

	for page <= lastPage {
		query.Set("page", fmt.Sprint(page))
		query.Set("perPage", "200")

		var slugsResponse organisersResponse

		req, err := http.NewRequestWithContext(ctx, "GET", m.url+"/api/organizers/"+kind, nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = query.Encode()
		log.Println("getting from", m.url+"/api/organizers/"+kind)

		m.addAuth(req)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
		}
		err = json.Unmarshal(body, &slugsResponse)
		if err != nil {
			log.Println("body", string(body))
			return nil, err
		}
		lastPage = slugsResponse.Pages
		slugs = append(slugs, slugsResponse.Items...)
		log.Printf("retrieved %d slugs from page %d", len(slugsResponse.Items), page)

		page++
	}

	log.Printf("retrieved %d slugs in total", len(slugs))
	return slugs, nil
}

type recipeForPatchingOrganisers struct {
	Categories []organiser `json:"recipeCategory"`
	Tags       []organiser `json:"tags"`
}

func (m *mealie) setOrganisers(ctx context.Context, recipe recipe) error {
	log.Printf("updating organisers for %s", recipe.Slug)

	converted := recipeForPatchingOrganisers{
		Categories: recipe.Categories,
		Tags:       recipe.Tags,
	}
	body, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("failed to convert organisers to json: %s", err.Error())
	}

	req, err := http.NewRequestWithContext(
		ctx, "PATCH", m.url+"/api/recipes/"+recipe.Slug, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to construct request")
	}
	req.Header.Add("Content-Type", "application/json")

	m.addAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %s", err.Error())
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %s", err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("updated organisers for %s", recipe.Slug)
	return nil
}
