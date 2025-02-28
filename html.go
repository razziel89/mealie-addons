// Package main contains the server code.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
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
	return g.pandoc.run(ctx, buildMarkdown(recipes, g.url), "html", buildTitle(timestamp), nil)
}

func removeAllHtmlElements(htmlInput []byte, element string) ([]byte, error) {
	root, err := html.Parse(bytes.NewReader(htmlInput))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML input: %s", err.Error())
	}

	nodesAtCurrentLevel := []*html.Node{root}
	nodesAtNextLevel := []*html.Node{}
	numRemoved := 0

	for len(nodesAtCurrentLevel) != 0 {
		for _, current := range nodesAtCurrentLevel {
			child := current.FirstChild
			for child != nil {
				next := child.NextSibling
				if child.Type == html.ElementNode && child.Data == element {
					numRemoved += 1
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
	log.Printf("removed %d nodes of type %s", numRemoved, element)

	return buf.Bytes(), nil
}

func redirectImgSources(htmlInput []byte, prefix string, newPrefix string) ([]byte, error) {
	element := "img"
	key := "src"
	root, err := html.Parse(bytes.NewReader(htmlInput))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML input: %s", err.Error())
	}

	nodesAtCurrentLevel := []*html.Node{root}
	nodesAtNextLevel := []*html.Node{}
	numReplaced := 0
	numKept := 0

	for len(nodesAtCurrentLevel) != 0 {
		for _, current := range nodesAtCurrentLevel {
			child := current.FirstChild
			for child != nil {
				next := child.NextSibling
				nodesAtNextLevel = append(nodesAtNextLevel, child)
				if child.Type == html.ElementNode && child.Data == element {
					replaced := false
					for idx := range child.Attr {
						attr := &child.Attr[idx]
						if attr.Key == key && strings.HasPrefix(attr.Val, prefix) {
							attr.Val = newPrefix + strings.TrimPrefix(attr.Val, prefix)
							replaced = true
						}
					}
					if replaced {
						numReplaced += 1
					} else {
						numKept += 1
					}
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
	log.Printf("redirected %d nodes of type %s", numReplaced, element)
	log.Printf("kept %d nodes of type %s", numReplaced, element)

	return buf.Bytes(), nil
}
