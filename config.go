// Package main contains the server code.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type config struct {
	mealieRetrievalURL string
	mealieBaseURL      string
	mealieToken        string
	selfURL            string
	listenInterface    string
	retrievalLimit     int
	timeoutSecs        int
	startupGraceSecs   int
	pandocFlags        []string
	pandocFontsDir     string
	imageAction        string
	htmlAttrsMod       map[string]map[string]string
	htmlAttrsRm        map[string]map[string]string
	queryAssignments   queryAssignments
	fixes              fixes
}

func initConfig() (cfg config, err error) {
	for _, env := range []string{
		"MEALIE_BASE_URL", "MEALIE_RETRIEVAL_URL", "MEALIE_TOKEN", "MA_LISTEN_INTERFACE",
		"MA_RETRIEVAL_LIMIT", "MA_STARTUP_GRACE_SECS", "MA_TIMEOUT_SECS",
	} {
		val := os.Getenv(env)
		if val == "" {
			err = fmt.Errorf("environment variable %s not defined or empty", env)
			return
		}
	}

	retrievalLimit, parseErr := strconv.Atoi(os.Getenv("MA_RETRIEVAL_LIMIT"))
	if parseErr != nil {
		err = parseErr
		return
	}
	startupGraceSecs, parseErr := strconv.Atoi(os.Getenv("MA_STARTUP_GRACE_SECS"))
	if parseErr != nil {
		err = parseErr
		return
	}
	timeoutSecs, parseErr := strconv.Atoi(os.Getenv("MA_TIMEOUT_SECS"))
	if parseErr != nil {
		err = parseErr
		return
	}
	interfaceEnv := os.Getenv("MA_LISTEN_INTERFACE")
	_, portStr, found := strings.Cut(interfaceEnv, ":")
	if !found {
		err = fmt.Errorf("cannot find port in interface spec %s", interfaceEnv)
		return
	}
	listenPort, parseErr := strconv.Atoi(portStr)
	if parseErr != nil {
		err = parseErr
		return
	}

	// Try to interpret the token as pointing to a file that exists. If so, we read the value from
	// the file. If not, we use the value from the environment directly. This enables the use of
	// docker-compose secrets.
	var token string
	tokenInput := os.Getenv("MEALIE_TOKEN")
	maybeToken, readErr := os.ReadFile(tokenInput)
	if readErr == nil {
		// It does point to a file.
		token = strings.TrimSpace(string(maybeToken))
	} else {
		token = strings.TrimSpace(tokenInput)
	}

	mealieBaseURL := os.Getenv("MEALIE_BASE_URL")
	// This block is used solely for backwards compatibility.
	if idx := strings.LastIndex(mealieBaseURL, "/g/"); idx != -1 {
		mealieBaseURL = mealieBaseURL[:idx]
	}

	pandocFlags := strings.Fields(os.Getenv("PANDOC_FLAGS"))

	pandocFontsDir := os.Getenv("PANDOC_FONTS_DIR")
	if pandocFontsDir == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			err = fmt.Errorf("failed to get current working directory: %s", cwdErr.Error())
			return
		}
		pandocFontsDir = cwd
	}

	imageAction := strings.ToLower(os.Getenv("MA_IMAGE_ACTION"))
	switch imageAction {
	case "":
		// The default action if none is set.
		imageAction = "remove"
	case "remove", "ignore", "embed":
	default:
		err = fmt.Errorf("unknown image action, must be 'ignore' or 'remove': %s", imageAction)
		return
	}

	htmlAttrsMod, parseErr := parseHtmlAttrs(os.Getenv("MA_HTML_ATTRS_MOD"))
	if parseErr != nil {
		err = parseErr
		return
	}

	htmlAttrsRm, parseErr := parseHtmlAttrs(os.Getenv("MA_HTML_ATTRS_RM"))
	if parseErr != nil {
		err = parseErr
		return
	}

	selfURL := os.Getenv("MA_SELF_URL")
	if selfURL == "" {
		selfURL = fmt.Sprintf("http://127.0.0.1:%d", listenPort)
	}

	var queryAssignments queryAssignments
	queryAssignmentsStr := os.Getenv("MA_QUERY_ASSIGNMENTS")
	if queryAssignmentsStr != "" {
		parseErr := json.Unmarshal([]byte(queryAssignmentsStr), &queryAssignments)
		if parseErr != nil {
			err = fmt.Errorf(
				"failed to parse MA_QUERY_ASSIGNMENTS as the expected JSON: %s",
				parseErr.Error(),
			)
			return
		}
		if queryAssignments.TimeoutSecs == 0 {
			err = fmt.Errorf("timeout-secs for query assignment must not be 0")
			return
		}
		if queryAssignments.RepeatSecs == 0 {
			err = fmt.Errorf("repeat-secs for query assignment must not be 0")
			return
		}
	}

	fixes, fixErr := fixesFromString(os.Getenv("MA_MEALIE_FIXES"))
	if fixErr != nil {
		err = fmt.Errorf("failed to parse fixes: %s", fixErr.Error())
		return
	}

	cfg = config{
		mealieRetrievalURL: os.Getenv("MEALIE_RETRIEVAL_URL"),
		mealieBaseURL:      mealieBaseURL,
		mealieToken:        token,
		selfURL:            selfURL,
		listenInterface:    interfaceEnv,
		retrievalLimit:     retrievalLimit,
		timeoutSecs:        timeoutSecs,
		startupGraceSecs:   startupGraceSecs,
		pandocFlags:        pandocFlags,
		pandocFontsDir:     pandocFontsDir,
		imageAction:        imageAction,
		htmlAttrsMod:       htmlAttrsMod,
		htmlAttrsRm:        htmlAttrsRm,
		queryAssignments:   queryAssignments,
		fixes:              fixes,
	}
	return
}
