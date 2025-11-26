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

// Package main contains the server code.
package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type markdownGenerator struct {
	url    string
	pandoc *pandoc
}

func (g *markdownGenerator) commonName() string {
	return "markdown"
}

func (g *markdownGenerator) extension() string {
	return "md"
}

func (g *markdownGenerator) mimeType() string {
	return "text/markdown"
}

func (g *markdownGenerator) response(
	ctx context.Context,
	recipes []recipe,
	timestamp time.Time,
) ([]byte, error) {
	htmlHook := func(htmlInput *html.Node) (*html.Node, error) {
		return removeAllHTMLElements(htmlInput, "img")
	}
	return g.pandoc.run(
		ctx,
		buildMarkdown(recipes, g.url),
		"markdown_github",
		buildTitle(timestamp),
		htmlHook,
	)
}

func buildTitle(timestamp time.Time) string {
	return fmt.Sprintf("Exported Recipes @ %s", timestamp.Format(time.RFC3339))
}

func buildMarkdown(recipes []recipe, url string) string {
	// Extract all known categories and tags to build the index at the end.
	tags := map[string]bool{}
	categories := map[string]bool{}
	for _, recipe := range recipes {
		for _, tag := range recipe.Tags {
			tags[tag.Name] = true
		}
		for _, category := range recipe.Categories {
			categories[category.Name] = true
		}
	}
	log.Printf("there are %d tags and %d categories overall", len(tags), len(categories))

	// Sort tags and categories for easier processing down the line.
	// Tags.
	sortedTags := make([]string, 0, len(tags))
	for tag := range tags {
		sortedTags = append(sortedTags, tag)
	}
	sort.Strings(sortedTags)
	// Categories.
	sortedCategories := make([]string, 0, len(categories))
	for category := range categories {
		sortedCategories = append(sortedCategories, category)
	}
	sort.Strings(sortedCategories)

	// Extract all tags and categories for each recipe. That makes it very easy to build the indices
	// down the line.
	// Tags.
	tagsPerRecipe := map[string][]string{}
	for _, recipe := range recipes {
		tags := make([]string, 0, len(recipe.Tags))
		for _, tag := range recipe.Tags {
			tags = append(tags, tag.Name)
		}
		tagsPerRecipe[recipe.ID] = tags
	}
	// Categories.
	categoriesPerRecipe := map[string][]string{}
	for _, recipe := range recipes {
		categories := make([]string, 0, len(recipe.Categories))
		for _, category := range recipe.Categories {
			categories = append(categories, category.Name)
		}
		categoriesPerRecipe[recipe.ID] = categories
	}

	result := []string{}

	// Recipes.
	result = append(result, "# Recipes")
	for _, recipe := range recipes {
		result = append(result, fmt.Sprintf("- [%s](#recipe-%s)", recipe.Name, recipe.ID))
	}
	result = append(result, "\n"+`<div style="page-break-before: always;"></div>`+"\n")
	for _, recipe := range recipes {
		result = append(result, recipeToMarkdown(&recipe, url)...)
	}

	// Tags index.
	tagsIndex := []string{`# Tags`}
	for _, tag := range sortedTags {
		tagsIndex = append(
			tagsIndex,
			fmt.Sprintf("\n## <a name=\"tag-%s\"></a> %s\n", slugify(tag), tag),
		)
		for _, recipe := range recipes {
			if slices.Contains(tagsPerRecipe[recipe.ID], tag) {
				link := fmt.Sprintf("- [%s](#recipe-%s)", recipe.Name, recipe.ID)
				tagsIndex = append(tagsIndex, link)
			}
		}
	}
	tagsIndex = append(tagsIndex, "\n"+`<div style="page-break-before: always;"></div>`+"\n")
	result = append(result, tagsIndex...)

	// Categories index.
	categoriesIndex := []string{`# Categories`}
	for _, category := range sortedCategories {
		categoriesIndex = append(
			categoriesIndex,
			fmt.Sprintf("\n## <a name=\"category-%s\"></a> %s\n", slugify(category), category),
		)
		for _, recipe := range recipes {
			if slices.Contains(categoriesPerRecipe[recipe.ID], category) {
				link := fmt.Sprintf("- [%s](#recipe-%s)", recipe.Name, recipe.ID)
				categoriesIndex = append(categoriesIndex, link)
			}
		}
	}
	categoriesIndex = append(
		categoriesIndex,
		"\n"+`<div style="page-break-before: always;"></div>`+"\n",
	)
	result = append(result, categoriesIndex...)

	return strings.Join(result, "\n")
}

func slugify(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(strings.ToLower(s))), "-")
}

func recipeToMarkdown(recipe *recipe, url string) []string {
	result := []string{}

	heading := fmt.Sprintf(`## <a name="recipe-%s"></a> %s

Total time: %s
`, recipe.ID, recipe.Name, recipe.TotalTime)
	result = append(result, heading)
	if len(recipe.Description) > 0 {
		result = append(result, fmt.Sprintf("%s\n", recipe.Description))
	}
	if len(recipe.Image) != 0 {
		result = append(
			result,
			fmt.Sprintf(
				"<img src=\"/api/media/recipes/%s/images/original.webp\" "+
					"alt=\"%s\" height=\"150\">\n",
				recipe.ID,
				strings.ReplaceAll(recipe.Name, `"`, " "),
			),
		)
	}
	result = append(
		result,
		"- **Go to**: [Recipes](#recipes), [Tags](#tags), [Categories](#categories), "+
			fmt.Sprintf("[Original](%s), ", recipe.OrgURL)+
			fmt.Sprintf("[Mealie](%s/r/%s)", url, recipe.Slug),
	)

	if len(recipe.Categories) > 0 {
		categories := make([]string, 0, len(recipe.Categories))
		for _, category := range recipe.Categories {
			categories = append(
				categories,
				fmt.Sprintf("[%s](#category-%s)", category.Name, slugify(category.Name)),
			)
		}
		categoriesStr := fmt.Sprintf("- **Categories**: %s", strings.Join(categories, ", "))
		result = append(result, categoriesStr)
	}

	if len(recipe.Tags) > 0 {
		tags := make([]string, 0, len(recipe.Tags))
		for _, tag := range recipe.Tags {
			tags = append(tags,
				fmt.Sprintf("[%s](#tag-%s)", tag.Name, slugify(tag.Name)),
			)
		}
		tagsStr := fmt.Sprintf("- **Tags**: %s", strings.Join(tags, ", "))
		result = append(result, tagsStr)
	}

	if len(recipe.Ingredients) > 0 {
		result = append(result, "- **Ingredients**:")
		for _, tmp := range recipe.Ingredients {
			result = append(result, fmt.Sprintf("    - %s", tmp.Text))
		}
	}

	if len(recipe.Instructions) > 0 {
		result = append(result, "- **Instructions**:")
		for _, tmp := range recipe.Instructions {
			result = append(result, fmt.Sprintf("    - %s", tmp.Text))
		}
	}

	if len(recipe.Comments) > 0 {
		result = append(result, "- **Comments**:")
		for _, tmp := range recipe.Comments {
			result = append(result, fmt.Sprintf("    - %s: %s", tmp.User.Name, tmp.Text))
		}
	}

	result = append(result, "\n"+`<div style="page-break-before: always;"></div>`+"\n")
	return result
}
