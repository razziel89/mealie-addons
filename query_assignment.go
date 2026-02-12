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

type queryAssignmentQuery struct {
	Params map[string]string `json:"params"`
	Mode   string            `json:"mode"`
}

type queryAssignment struct {
	Queries    []queryAssignmentQuery `json:"queries"`
	Categories queryAssignmentData    `json:"categories"`
	Tags       queryAssignmentData    `json:"tags"`
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
						skipThis := false
						// Check whether all referenced tags and categories are known.
						for _, category := range assignment.Categories.Set {
							if !slices.Contains(categories, category) {
								log.Printf(
									"skipping assignment %d, category %s not known",
									assignmentIdx+1,
									category,
								)
								skipThis = true
							}
						}
						for _, category := range assignment.Categories.Unset {
							if !slices.Contains(categories, category) {
								log.Printf(
									"skipping assignment %d, category %s not known",
									assignmentIdx+1,
									category,
								)
								skipThis = true
							}
						}
						for _, tag := range assignment.Tags.Set {
							if !slices.Contains(tags, tag) {
								log.Printf(
									"skipping assignment %d, tag %s not known",
									assignmentIdx+1,
									tag,
								)
								skipThis = true
							}
						}
						for _, tag := range assignment.Tags.Unset {
							if !slices.Contains(tags, tag) {
								log.Printf(
									"skipping assignment %d, tag %s not known",
									assignmentIdx+1,
									tag,
								)
								skipThis = true
							}
						}
						if skipThis {
							continue
						}

						recipeSlugsRetention := map[slug]bool{}
						ctx, cancel = context.WithTimeout(background, timeout)
						for queryIdx, query := range assignment.Queries {
							// Check whether this query's mode is known.
							switch query.Mode {
							case "add", "remove":
								// Retrieve recipe slugs that match this query.
								queryVals := url.Values{}
								for key, value := range query.Params {
									queryVals.Add(key, value)
								}
								log.Printf(
									"built string for query %d of assignment %d: %v",
									queryIdx+1,
									assignmentIdx+1,
									&queryVals,
								)
								querySlugs, err := mealie.getSlugs(ctx, &queryVals)
								if err != nil {
									log.Printf("failed to retrieve recipes: %s", err.Error())
									continue
								}
								log.Printf(
									"%d recipes matched query %d of assignment %d in mode %s",
									len(querySlugs),
									queryIdx+1,
									assignmentIdx+1,
									query.Mode,
								)
								if query.Mode == "add" {
									for _, slug := range querySlugs {
										recipeSlugsRetention[slug] = true
									}
								} else {
									for _, slug := range querySlugs {
										recipeSlugsRetention[slug] = false
									}
								}
							case "skip":
								log.Printf(
									"skipping query %d of assignment %d due to mode setting",
									queryIdx+1,
									assignmentIdx+1,
								)
								continue
							default:
								log.Printf(
									"skipping query %d of assignment %d, unknown mode %s",
									queryIdx+1,
									assignmentIdx+1,
									query.Mode,
								)
								continue
							}
						}
						cancel()

						recipeSlugs := make([]slug, 0, len(recipeSlugsRetention))
						for slug, keep := range recipeSlugsRetention {
							if keep {
								recipeSlugs = append(recipeSlugs, slug)
							}
						}

						// Assign everything for each matched recipe.
						numSlugs := len(recipeSlugs)
						if numSlugs == 0 {
							log.Printf(
								"No recipes to process for assignment %d/%d",
								assignmentIdx+1,
								numAssignments,
							)
						}
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
