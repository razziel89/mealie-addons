// Package main contains the server code.
package main

import (
	"context"
	"time"
)

type epubGenerator struct {
	url    string
	pandoc *pandoc
}

func (g *epubGenerator) commonName() string {
	return "epub"
}

func (g *epubGenerator) extension() string {
	return "epub"
}

func (g *epubGenerator) mimeType() string {
	return "application/epub+zip"
}

func (g *epubGenerator) response(
	ctx context.Context,
	recipes []recipe,
	timestamp time.Time,
) ([]byte, error) {
	return g.pandoc.run(ctx, buildMarkdown(recipes, g.url), "epub", buildTitle(timestamp))
}
