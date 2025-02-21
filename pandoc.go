// Package main contains the server code.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
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
	"--variable=mainfont:notosans.ttf",
	"--variable=mainfontfallback:[notosanssc.ttf]",
	"--variable=mainfontfallback:[notosanssymbols.ttf]",
	`--variable=header-includes:\usepackage[utf8x]{inputenc}`,
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
	options []string
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
	args := append([]string{}, defaultPandocOptions...)
	args = append(args, p.options...)
	args = append(args, "--from=markdown", "--to=html", "--output=-", "-")

	html, errMsg, err := runExe(ctx, "pandoc", args, nil, []byte(markdownInput))
	if errMsg != "" {
		log.Println("stderr when running pandoc:", errMsg)
	}
	if err != nil {
		return nil, err
	}
	// Convert again, but to the desired format.
	var converted []byte
	if toFormat != "html" {
		args = append(args, "--from=html", "--to", toFormat)
		converted, errMsg, err = runExe(ctx, "pandoc", args, nil, html)
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
