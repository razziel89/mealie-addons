// Package main contains the server code.
package main

import (
	"context"
	"time"
)

type htmlGenerator struct {
	url string
}

func (g *htmlGenerator) commonName() string {
	return "html"
}

func (g *htmlGenerator) extension() string {
	return "html"
}

func (g *htmlGenerator) mimeType() string {
	return "text/html"
}

func (g *htmlGenerator) response(
	ctx context.Context,
	recipes []recipe,
	timestamp time.Time,
) ([]byte, error) {
	return runPandoc(ctx, buildMarkdown(recipes, g.url, timestamp), "html")
}
