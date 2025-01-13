// Package main contains the server code.
package main

import (
	"fmt"
	"os"
	"strconv"
)

type config struct {
	mealieRetrievalURL string
	mealieBaseURL      string
	mealieToken        string
	listenInterface    string
	retrievalLimit     int
	timeoutSecs        int
	startupGraceSecs   int
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

	cfg = config{
		mealieRetrievalURL: os.Getenv("MEALIE_RETRIEVAL_URL"),
		mealieBaseURL:      os.Getenv("MEALIE_BASE_URL"),
		mealieToken:        os.Getenv("MEALIE_TOKEN"),
		listenInterface:    os.Getenv("MA_LISTEN_INTERFACE"),
		retrievalLimit:     retrievalLimit,
		timeoutSecs:        timeoutSecs,
		startupGraceSecs:   startupGraceSecs,
	}
	return
}
