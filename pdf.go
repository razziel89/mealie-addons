// Package main contains the server code.
package main

import (
	"context"
	"time"
)

type pdfGenerator struct {
	url    string
	pandoc *pandoc
}

func (g *pdfGenerator) commonName() string {
	return "pdf"
}

func (g *pdfGenerator) extension() string {
	return "pdf"
}

func (g *pdfGenerator) mimeType() string {
	return "application/pdf"
}

func (g *pdfGenerator) response(
	ctx context.Context,
	recipes []recipe,
	timestamp time.Time,
) ([]byte, error) {
	return g.pandoc.run(ctx, buildMarkdown(recipes, g.url, timestamp), "pdf")
}
