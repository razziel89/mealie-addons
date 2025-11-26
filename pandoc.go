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
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var defaultPandocAlwaysArgs = []string{
	"--verbose",
	"--output=-",
	"-",
}

var defaultPandocFirstArgs = []string{
	"--from=markdown",
	"--to=html5",
}

var defaultPandocLastArgs = []string{
	"--from=html",
	"--standalone",
	"--embed-resources",
	"--pdf-engine=lualatex",
	"--variable=geometry:margin=2cm",
	"--table-of-contents=true",
	"--epub-title-page=false",
}

// Call an executable with arguments and return stdout and stderr. Specify the executable via
// "exe"", the arguments via "args", additional environment variables in the form "key=value" via
// "env", and standard input via "stdin". The command will be cancelled automatically when the
// context expires.
func runExe(
	ctx context.Context, exe string, args []string, env []string, stdin []byte,
) ([]byte, string, error) {
	log.Println("running", exe, "with args:", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = env

	cmd.Stdin = bytes.NewReader(stdin)

	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout
	stderr := strings.Builder{}
	cmd.Stderr = &stderr

	err := cmd.Run()

	return stdout.Bytes(), stderr.String(), err
}

type pandoc struct {
	options       []string
	mainFont      string
	fallbackFonts []string
	htmlHooks     []func(*html.Node) (*html.Node, error)
}

func (p *pandoc) loadFonts(dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of %s: %s", dir, err.Error())
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %s", err.Error())
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of %s: %s", cwd, err.Error())
	}
	doCopy := cwd != dir

	content, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to list directory %s: %s", dir, err.Error())
	}
	filtered := make([]string, 0, len(content))
	for _, file := range content {
		isRelevant := false
		if file.Name() == "main.ttf" {
			p.mainFont = "--variable=mainfont:" + file.Name()
			isRelevant = true
		} else if strings.HasSuffix(file.Name(), ".ttf") {
			arg := fmt.Sprintf("--variable=mainfontfallback:[%s]", file.Name())
			filtered = append(filtered, arg)
			isRelevant = true
		}
		if doCopy && isRelevant {
			err = copyFile(filepath.Join(dir, file.Name()), filepath.Join(cwd, file.Name()))
			if err != nil {
				return fmt.Errorf(
					"failed to copy relevant font file %s/%s: %s",
					dir, file.Name(), err.Error(),
				)
			}
		}
	}
	slices.Sort(filtered)
	if len(filtered) != 0 {
		p.fallbackFonts = filtered
	}
	return nil
}

func copyFile(source string, destination string) error {
	data, err := os.ReadFile(source) //#nosec:G304
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %s", source, err.Error())
	}
	err = os.WriteFile(destination, data, 0o600) //nolint:mnd
	if err != nil {
		return fmt.Errorf("failed to write destination file %s: %s", destination, err.Error())
	}
	return nil
}

func checkForPandoc() error {
	_, err := exec.LookPath("pandoc")
	if err != nil {
		return fmt.Errorf("failed to find pandoc in path: %s", err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) //nolint:mnd
	defer cancel()
	output, _, err := runExe(
		ctx,
		"pandoc",
		[]string{"--version"},
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to run pandoc --version: %s", err.Error())
	}
	log.Printf("pandoc version information:\n%s", output)
	return nil
}

// We convert twice for anything that isn't HTML. The reason is that links in the document are
// broken unless we first convert to HTML, but if we do that, they work also for other formats. No
// clue why that is.
func (p *pandoc) run(
	ctx context.Context,
	markdownInput string,
	toFormat string,
	title string,
	filetypeHook func(*html.Node) (*html.Node, error),
) ([]byte, error) {
	alwaysArgs := append([]string{}, defaultPandocAlwaysArgs...)
	alwaysArgs = append(alwaysArgs, "--metadata", "title="+title, "--metadata", "pagetitle="+title)
	alwaysUserArgs := []string{}
	for _, arg := range p.options {
		if !strings.HasPrefix(arg, "@first:") && !strings.HasPrefix(arg, "@last:") {
			alwaysUserArgs = append(alwaysUserArgs, arg)
		}
	}

	// Convert to HTML first. Somehow, internal links are broken without doing so.
	firstArgs := append([]string{}, alwaysUserArgs...)
	for _, arg := range p.options {
		if rest, found := strings.CutPrefix(arg, "@first:"); found {
			firstArgs = append(firstArgs, rest)
		}
	}
	firstArgs = append(firstArgs, alwaysArgs...)
	firstArgs = append(firstArgs, defaultPandocFirstArgs...)
	firstArgs = append(firstArgs, "--metadata", "title="+title, "--metadata", "pagetitle="+title)

	htmlIntermediate, errMsg, err := runExe(ctx, "pandoc", firstArgs, nil, []byte(markdownInput))
	if errMsg != "" {
		log.Println("stderr when running pandoc:", errMsg)
	}
	if err != nil {
		return nil, err
	}

	root, err := html.Parse(bytes.NewReader(htmlIntermediate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated html: %s", err.Error())
	}
	for idx, hook := range p.htmlHooks {
		root, err = hook(root)
		if err != nil {
			return nil, fmt.Errorf("failed to run %d'nth html hook: %s", idx+1, err.Error())
		}
	}
	if filetypeHook != nil {
		root, err = filetypeHook(root)
		if err != nil {
			return nil, fmt.Errorf("failed to run filetype html hook: %s", err.Error())
		}
	}
	buf := bytes.Buffer{}
	err = html.Render(&buf, root)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML output: %s", err.Error())
	}
	htmlIntermediate = buf.Bytes()

	// Convert again, but to the desired format.
	lastArgs := append([]string{}, alwaysUserArgs...)
	for _, arg := range p.options {
		if rest, found := strings.CutPrefix(arg, "@last:"); found {
			lastArgs = append(lastArgs, rest)
		}
	}
	if p.mainFont != "" {
		lastArgs = append(lastArgs, p.mainFont)
	}
	if p.fallbackFonts != nil {
		lastArgs = append(lastArgs, p.fallbackFonts...)
	}
	lastArgs = append(lastArgs, alwaysArgs...)
	lastArgs = append(lastArgs, defaultPandocLastArgs...)
	lastArgs = append(lastArgs, "--to", toFormat)

	converted, errMsg, err := runExe(ctx, "pandoc", lastArgs, nil, htmlIntermediate)
	if errMsg != "" {
		log.Println("stderr when running pandoc:", errMsg)
	}
	if err != nil {
		return nil, err
	}
	return converted, nil
}
