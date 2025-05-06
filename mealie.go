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
	ID           string         `json:"id"`
	Slug         string         `json:"slug"`
	Name         string         `json:"name"`
	Servings     float32        `json:"recipeServings"`
	TotalTime    string         `json:"totalTime"`
	Description  string         `json:"description"`
	OrgURL       string         `json:"orgURL"`
	Categories   []*category    `json:"recipeCategory"`
	Tags         []*tag         `json:"tags"`
	Instructions []*instruction `json:"recipeInstructions"`
	Ingredients  []*ingredient  `json:"recipeIngredient"`
	Comments     []*comment     `json:"comments"`
	Image        string         `json:"image"`
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

type category struct {
	Text string `json:"name"`
}

func (c *category) normalise() {
	c.Text = collapseWhitespace(c.Text)
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

type tag struct {
	Text string `json:"name"`
}

func (t *tag) normalise() {
	t.Text = collapseWhitespace(t.Text)
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
	Slug       string      `json:"slug"`
	Categories []*category `json:"recipeCategory"`
	Tags       []*tag      `json:"tags"`
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

	page := 1
	lastPage := 10
	var slugs []slug

	for page <= lastPage {
		query.Set("page", fmt.Sprint(page))
		query.Set("per_page", "200")

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
		idx := idx
		slug := slug
		// Retrieve all recipes in parallel. Let'ssee if this works.
		go func() {
			if m.limiter != nil {
				m.limiter <- true
			}
			recipe, err := m.getRecipe(ctx, slug.Slug)
			if err == nil {
				recipe.normalise()
				recipes[idx] = recipe
			} else {
				errs[idx] = err
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

func (m mealie) addAuth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.token))
}

func (m mealie) check() (group string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

type organiser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	kind string
}

func (m *mealie) getOrganisers(ctx context.Context, kind string) ([]organiser, error) {
	if kind != "categories" && kind != "tags" {
		return nil, fmt.Errorf("Can only get categories or tags for now but not '%s'.", kind)
	}
	log.Printf("getting %s", kind)

	page := 1
	lastPage := 10
	var slugs []organiser
	query := url.Values{}

	for page <= lastPage {
		query.Set("page", fmt.Sprint(page))
		query.Set("per_page", "200")

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

	for idx := range slugs {
		slugs[idx].kind = kind
	}

	log.Printf("retrieved %d slugs in total", len(slugs))
	return slugs, nil
}
