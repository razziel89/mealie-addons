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
	return g.pandoc.run(ctx, buildMarkdown(recipes, g.url), "pdf", buildTitle(timestamp), nil)
}
