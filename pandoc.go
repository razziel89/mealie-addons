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
)

var defaultPandocOptions = []string{
	"--verbose",
	"--verbose",
	"--verbose",
	"--standalone",
	"--pdf-engine=lualatex",
	"--variable=geometry:margin=2cm",
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
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %s", source, err.Error())
	}
	err = os.WriteFile(destination, data, 0644)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
func (p *pandoc) run(ctx context.Context, markdownInput string, toFormat string) ([]byte, error) {
	alwaysArgs := append([]string{}, defaultPandocOptions...)
	if p.mainFont != "" {
		alwaysArgs = append(alwaysArgs, p.mainFont)
	}
	if p.fallbackFonts != nil {
		alwaysArgs = append(alwaysArgs, p.fallbackFonts...)
	}
	for _, arg := range p.options {
		if !strings.HasPrefix(arg, "@first:") && !strings.HasPrefix(arg, "@last:") {
			alwaysArgs = append(alwaysArgs, arg)
		}
	}

	firstArgs := append([]string{}, alwaysArgs...)
	for _, arg := range p.options {
		if strings.HasPrefix(arg, "@first:") {
			firstArgs = append(firstArgs, strings.TrimPrefix(arg, "@first:"))
		}
	}
	firstArgs = append(firstArgs, "--from=markdown", "--to=html", "--output=-", "-")

	html, errMsg, err := runExe(ctx, "pandoc", firstArgs, nil, []byte(markdownInput))
	if errMsg != "" {
		log.Println("stderr when running pandoc:", errMsg)
	}
	if err != nil {
		return nil, err
	}
	// Convert again, but to the desired format.
	var converted []byte
	if toFormat != "html" {
		lastArgs := append([]string{}, alwaysArgs...)
		for _, arg := range p.options {
			if strings.HasPrefix(arg, "@last:") {
				lastArgs = append(lastArgs, strings.TrimPrefix(arg, "@last:"))
			}
		}
		lastArgs = append(lastArgs, "--from=html", "--to", toFormat, "--output=-", "-")

		converted, errMsg, err = runExe(ctx, "pandoc", lastArgs, nil, html)
		if errMsg != "" {
			log.Println("stderr when running pandoc:", errMsg)
		}
		if err != nil {
			return nil, err
		}
	} else {
		converted = html
	}
	return converted, nil
}
