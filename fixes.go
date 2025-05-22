// Package main contains the server code.
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
)

func reuploadImages(mealie *mealie) error {
	log.Printf("reuploading images")

	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	// defer cancel()
	ctx := context.Background()
	counter := 0

	query := url.Values{}
	query.Add("queryFilter", "image IS NOT NULL")
	slugs, err := mealie.getSlugs(ctx, &query)
	if err != nil {
		return fmt.Errorf("failed to retrieve slugs for image-reupload: %s", err.Error())
	}

	for _, slug := range slugs {
		reuploaded, err := mealie.reuploadImage(ctx, slug.Slug)
		if err != nil {
			return fmt.Errorf("failed to reupload image for %s: %s", slug.Slug, err.Error())
		}
		if reuploaded {
			counter++
		}
	}

	log.Printf("reuploaded images for %d recipes", counter)
	return nil
}
