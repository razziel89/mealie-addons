// Package main contains the server code.
package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"golang.org/x/net/html"
)

type htmlGenerator struct {
	url    string
	pandoc *pandoc
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
	return g.pandoc.run(ctx, buildMarkdown(recipes, g.url), "html", buildTitle(timestamp))
}

func removeAllHtmlElements(htmlInput []byte, element string) ([]byte, error) {
	root, err := html.Parse(bytes.NewReader(htmlInput))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML input: %s", err.Error())
	}

	nodesAtCurrentLevel := []*html.Node{root}
	nodesAtNextLevel := []*html.Node{}

	for len(nodesAtCurrentLevel) != 0 {
		for _, current := range nodesAtCurrentLevel {
			child := current.FirstChild
			for child != nil {
				next := child.NextSibling
				if child.Type == html.ElementNode && child.Data == element {
					current.RemoveChild(child)
				} else {
					nodesAtNextLevel = append(nodesAtNextLevel, child)
				}
				child = next
			}
		}
		nodesAtCurrentLevel = nodesAtNextLevel
		nodesAtNextLevel = []*html.Node{}
	}

	buf := bytes.Buffer{}
	err = html.Render(&buf, root)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML output: %s", err.Error())
	}

	return buf.Bytes(), nil
}
