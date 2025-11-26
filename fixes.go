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
	"net/url"
	"strings"
)

type fixes struct {
	imageReupload bool
}

func fixesFromString(s string) (fixes, error) {
	fixes := fixes{}
	for fix := range strings.FieldsSeq(s) {
		switch fix {
		case "image-reupload":
			fixes.imageReupload = true
		default:
			return fixes, fmt.Errorf("unknown fix %s", fix)
		}
	}
	return fixes, nil
}

func reuploadImages(mealie *mealie) error {
	log.Printf("reuploading images")

	ctx := context.Background()
	counter := 0

	query := url.Values{}
	query.Add("queryFilter", "image IS NULL")
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
