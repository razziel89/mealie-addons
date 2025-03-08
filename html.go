// Package main contains the server code.
package main

import (
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

func removeAllHtmlElements(root *html.Node, element string) (*html.Node, error) {
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

	log.Printf("removed %d nodes of type %s", numRemoved, element)
	return root, nil
}

func redirectImgSources(root *html.Node, prefix string, newPrefix string) (*html.Node, error) {
	element := "img"
	key := "src"

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

	log.Printf("redirected %d nodes of type %s", numReplaced, element)
	log.Printf("kept %d nodes of type %s", numReplaced, element)

	return root, nil
}

func updateHtmlAttrs(
	root *html.Node,
	mapMod map[string]map[string]string,
	mapRm map[string]map[string]string,
) (*html.Node, error) {
	nodesAtCurrentLevel := []*html.Node{root}
	nodesAtNextLevel := []*html.Node{}
	numMod := 0
	numRm := 0

	for len(nodesAtCurrentLevel) != 0 {
		for _, current := range nodesAtCurrentLevel {
			child := current.FirstChild
			for child != nil {
				next := child.NextSibling
				nodesAtNextLevel = append(nodesAtNextLevel, child)
				if child.Type == html.ElementNode {
					mod, found := mapMod[child.Data]
					if found {
						didModify := map[string]bool{}
						for idx := range child.Attr {
							attr := &child.Attr[idx]
							if newVal, found := mod[attr.Key]; found {
								didModify[attr.Key] = true
								attr.Val = newVal
								log.Printf(
									"setting html attribute for %s: %s=%s (was %s)",
									child.Data, attr.Key, newVal, attr.Val,
								)
								numMod++
							}
						}
						for key, val := range mod {
							if _, found := didModify[key]; !found {
								log.Printf(
									"adding html attribute for %s: %s=%s",
									child.Data, key, val,
								)
								child.Attr = append(child.Attr, html.Attribute{Key: key, Val: val})
							}
						}
					}
					rm, found := mapRm[child.Data]
					if found {
						newAttrs := make([]html.Attribute, 0, len(child.Attr))
						for _, attr := range child.Attr {
							if _, found := rm[attr.Key]; !found {
								newAttrs = append(newAttrs, attr)
							} else {
								numRm++
								log.Printf(
									"removing html attribute for %s: %s (was %s)",
									child.Data, attr.Key, attr.Val,
								)
							}
						}
						child.Attr = newAttrs
					}
				}
				child = next
			}
		}
		nodesAtCurrentLevel = nodesAtNextLevel
		nodesAtNextLevel = []*html.Node{}
	}

	log.Printf("modified %d html attributes", numMod)
	log.Printf("removed %d html attributes", numRm)

	return root, nil
}

func parseHtmlAttrs(htmlInput string) (map[string]map[string]string, error) {
	result := map[string]map[string]string{}
	if htmlInput == "" {
		return result, nil
	}

	root, err := html.Parse(strings.NewReader(htmlInput))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML input: %s", err.Error())
	}

	nodesAtCurrentLevel := []*html.Node{root}
	nodesAtNextLevel := []*html.Node{}

	for len(nodesAtCurrentLevel) != 0 {
		for _, current := range nodesAtCurrentLevel {
			child := current.FirstChild
			for child != nil {
				nodesAtNextLevel = append(nodesAtNextLevel, child)
				if child.Type == html.ElementNode {
					elementResult, found := result[child.Data]
					if !found {
						elementResult = map[string]string{}
					}
					for _, attr := range child.Attr {
						elementResult[attr.Key] = attr.Val
					}
					result[child.Data] = elementResult
				}
				child = child.NextSibling
			}
		}
		nodesAtCurrentLevel = nodesAtNextLevel
		nodesAtNextLevel = []*html.Node{}
	}

	numElems := len(result)
	numAttrs := 0
	for _, elementResult := range result {
		numAttrs += len(elementResult)
	}

	keys := []string{}
	for key := range result {
		keys = append(keys, key)
	}

	for _, key := range keys {
		if len(result[key]) == 0 {
			delete(result, key)
		}
	}

	log.Printf("parsed html into %d elements and %d attributes", numElems, numAttrs)
	return result, nil
}
