package main

import (
	"context"
	"log"
	"net/url"
	"slices"
	"strings"
	"time"
)

type queryAssignmentData struct {
	Set   []string `json:"set"`
	Unset []string `json:"unset"`
}

type queryAssignment struct {
	Query      map[string]string   `json:"query"`
	Categories queryAssignmentData `json:"categories"`
	Tags       queryAssignmentData `json:"tags"`
}

type queryAssignments struct {
	RepeatSecs  int               `json:"repeat-secs"`
	TimeoutSecs int               `json:"timeout-secs"`
	Assignments []queryAssignment `json:"assignments"`
}

func updateSlice[T comparable](original []T, add []T, remove []T) ([]T, bool) {
	wasChanged := false

	asMap := make(map[T]bool, len(original))
	for _, org := range original {
		asMap[org] = true
	}

	length := len(asMap)
	for _, addThis := range add {
		asMap[addThis] = true
	}
	wasChanged = length != len(asMap)

	length = len(asMap)
	for _, rmThis := range remove {
		delete(asMap, rmThis)
	}
	wasChanged = wasChanged || length != len(asMap)

	asSlice := make([]T, 0, len(asMap))
	for key := range asMap {
		asSlice = append(asSlice, key)
	}
	return asSlice, wasChanged
}

func indexedSlice[T comparable](myMap map[string]T, indices []string) []T {
	result := make([]T, 0, len(indices))
	for _, index := range indices {
		if value, found := myMap[index]; found {
			result = append(result, value)
		}
	}
	return result
}

func launchAssignmentLoop(assignments queryAssignments, mealie *mealie) (chan<- bool, error) {
	// Perform sanity checks first.
	if len(assignments.Assignments) == 0 {
		return nil, nil
	}

	background := context.Background()
	timeout := time.Duration(assignments.TimeoutSecs) * time.Second
	repeatTime := time.Duration(assignments.RepeatSecs) * time.Second
	nextWaitTime := time.Duration(0)

	quit := make(chan bool)

	go func() {
		for {
			select {
			case <-quit:
				return
			case <-time.After(nextWaitTime):
				startTime := time.Now()
				skipAll := false

				// Handle categories. First retrieval.
				ctx, cancel := context.WithTimeout(background, timeout)
				categoriesRaw, err := mealie.getOrganisers(ctx, "categories")
				if err != nil {
					skipAll = true
					log.Printf("failed to retrieve categories: %s", err.Error())
				}
				cancel()
				// Then conversion to a nicer data structure.
				categories := make([]string, 0, len(categoriesRaw))
				categoriesMap := make(map[string]organiser, len(categoriesRaw))
				for _, category := range categoriesRaw {
					categories = append(categories, category.Name)
					categoriesMap[category.Name] = category
				}
				// Then logging.
				log.Printf("known categories: %s", strings.Join(categories, ", "))

				// Handle tags. First retrieval.
				ctx, cancel = context.WithTimeout(background, timeout)
				tagsRaw, err := mealie.getOrganisers(ctx, "tags")
				if err != nil {
					skipAll = true
					log.Printf("failed to retrieve tags: %s", err.Error())
				}
				cancel()
				// Then conversion to a nicer data structure.
				tags := make([]string, 0, len(tagsRaw))
				tagsMap := make(map[string]organiser, len(categoriesRaw))
				for _, tag := range tagsRaw {
					tags = append(tags, tag.Name)
					tagsMap[tag.Name] = tag
				}
				// Then logging.
				log.Printf("known tags: %s", strings.Join(tags, ", "))

				if !skipAll {
					// Perform actions for each assignment.
					numAssignments := len(assignments.Assignments)
					for assignmentIdx, assignment := range assignments.Assignments {
						// Check whether all referenced tags and categories are known.
						skipThis := false
						for _, category := range assignment.Categories.Set {
							if !slices.Contains(categories, category) {
								log.Printf(
									"skipping assignment %d, category %s not known",
									assignmentIdx,
									category,
								)
								skipThis = true
							}
						}
						for _, category := range assignment.Categories.Unset {
							if !slices.Contains(categories, category) {
								log.Printf(
									"skipping assignment %d, category %s not known",
									assignmentIdx,
									category,
								)
								skipThis = true
							}
						}
						for _, tag := range assignment.Tags.Set {
							if !slices.Contains(tags, tag) {
								log.Printf(
									"skipping assignment %d, tag %s not known",
									assignmentIdx,
									tag,
								)
								skipThis = true
							}
						}
						for _, tag := range assignment.Tags.Unset {
							if !slices.Contains(tags, tag) {
								log.Printf(
									"skipping assignment %d, tag %s not known",
									assignmentIdx,
									tag,
								)
								skipThis = true
							}
						}
						if skipThis {
							continue
						}

						// Retrieve recipe slugs that match this query.
						query := url.Values{}
						for key, value := range assignment.Query {
							query.Add(key, value)
						}
						log.Println("built query string", &query)
						ctx, cancel = context.WithTimeout(background, timeout)
						recipeSlugs, err := mealie.getSlugs(ctx, &query)
						cancel()
						if err != nil {
							log.Printf("failed to retrieve recipes: %s", err.Error())
							continue
						}
						log.Printf("%d recipes matched query %d", len(recipeSlugs), assignmentIdx)

						// Assign everything for each matched recipe.
						numSlugs := len(recipeSlugs)
						for slugIdx, slug := range recipeSlugs {
							log.Printf(
								"processing recipe %d/%d for assignment %d/%d",
								slugIdx+1, numSlugs, assignmentIdx+1, numAssignments,
							)
							ctx, cancel = context.WithTimeout(background, timeout)
							recipe, err := mealie.getRecipe(ctx, slug.Slug)
							cancel()
							if err != nil {
								log.Printf(
									"skipping recipe %s that failed to yield details: %s",
									slug, err.Error(),
								)
								continue
							}
							var categoriesChanged, tagsChanged bool
							recipe.Categories, categoriesChanged = updateSlice(
								recipe.Categories,
								indexedSlice(categoriesMap, assignment.Categories.Set),
								indexedSlice(categoriesMap, assignment.Categories.Unset),
							)
							recipe.Tags, tagsChanged = updateSlice(
								recipe.Tags,
								indexedSlice(tagsMap, assignment.Tags.Set),
								indexedSlice(tagsMap, assignment.Tags.Unset),
							)
							if categoriesChanged || tagsChanged {
								ctx, cancel = context.WithTimeout(background, timeout)
								err = mealie.setOrganisers(ctx, recipe)
								cancel()
								if err != nil {
									log.Printf("failed to update organisers: %s", err.Error())
								}
							}
						}
					}
				}
				timePassed := time.Since(startTime)
				nextWaitTime = max(repeatTime-timePassed, 0)
			}
		}
	}()

	return quit, nil
}
