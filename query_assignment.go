package main

import (
	"context"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"
)

type queryAssignmentData struct {
	Set   []string
	Unset []string
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

// // An example JSON config follows:
// {
//     "repeat-secs": 30,
//     "timeout-secs": 30,
//     "assignments": [
//         {
//             "query": {
//                 "queryFilter": "lastMade IS NOT NULL"
//             },
//             "categories": {
//                 "set": ["Made"],
//                 "unset": ["NotMade"]
//             },
//             "tags": {
//                 "set": ["Yummy", "Unknown"],
//                 "unset": []
//             }
//         },
//         {
//             "query": {
//                 "queryFilter": "lastMade IS NULL"
//             },
//             "categories": {
//                 "set": ["NotMade"],
//                 "unset": ["Made"]
//             },
//             "tags": {
//                 "set": ["Unknown"],
//                 "unset": ["Yummy"]
//             }
//         }
//     ]
// }

func launchAssignmentLoop(assignments queryAssignments, mealie mealie) (chan<- bool, error) {
	// Perform sanity checks first.
	if len(assignments.Assignments) == 0 {
		return nil, nil
	}

	back := context.Background()
	timeout := time.Duration(assignments.TimeoutSecs) * time.Second
	_ = time.Duration(assignments.RepeatSecs) * time.Second

	// Handle categories.
	// First retrieval.
	ctx, cancel := context.WithTimeout(back, timeout)
	categoriesRaw, err := mealie.getOrganisers(ctx, "categories")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to retrieve categories: %s", err.Error())
	}
	cancel()
	// Then conversion to a nicer data structure.
	categories := make(map[string]string, len(categoriesRaw))
	for _, category := range categoriesRaw {
		categories[category.Name] = category.ID
	}
	// Then logging.
	keys := slices.Sorted(maps.Keys(categories))
	log.Printf("known categories: %s", strings.Join(keys, ", "))

	// Handle tags.
	// First retrieval.
	ctx, cancel = context.WithTimeout(back, timeout)
	tagsRaw, err := mealie.getOrganisers(ctx, "tags")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to retrieve tags: %s", err.Error())
	}
	cancel()
	// Then conversion to a nicer data structure.
	tags := make(map[string]string, len(tagsRaw))
	for _, tag := range tagsRaw {
		tags[tag.Name] = tag.ID
	}
	// Then logging.
	keys = slices.Sorted(maps.Keys(categories))
	log.Printf("known tags: %s", strings.Join(keys, ", "))

	// Check whether all referenced tags and categories are known.
	for _, assignment := range assignments.Assignments {
		for _, category := range assignment.Categories.Set {
			if _, found := categories[category]; !found {
				return nil, fmt.Errorf("category %s not known", category)
			}
		}
		for _, category := range assignment.Categories.Unset {
			if _, found := categories[category]; !found {
				return nil, fmt.Errorf("category %s not known", category)
			}
		}
		for _, tag := range assignment.Tags.Set {
			if _, found := tags[tag]; !found {
				return nil, fmt.Errorf("tag %s not known", tag)
			}
		}
		for _, tag := range assignment.Tags.Unset {
			if _, found := tags[tag]; !found {
				return nil, fmt.Errorf("tag %s not known", tag)
			}
		}
	}

	quit := make(chan bool)

	go func() {
		select {
		case <-quit:
			return
		case <-time.After(time.Second * time.Duration(assignments.TimeoutSecs)):
		}
	}()

	return quit, nil
}
