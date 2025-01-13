// Package main contains the server code.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Initialise everything.
func main() {
	quit := make(chan bool)
	var err error

	// Config.
	var cfg config
	if cfg, err = initConfig(); err != nil {
		log.Fatalf("config not sane: %s", err.Error())
	}
	if err := checkForPandoc(); err != nil {
		log.Fatalf("missing executable: %s", err.Error())
	}

	mealie := mealie{url: cfg.mealieRetrievalURL, token: cfg.mealieToken, limit: cfg.retrievalLimit}
	works, try := false, 1
	for !works && try <= cfg.startupGraceSecs {
		err := mealie.check()
		if err != nil {
			log.Printf(
				"cannot connect to mealie, retrying at most %d times every 1s: %s",
				cfg.startupGraceSecs-try,
				err.Error(),
			)
			time.Sleep(time.Second)
		}
		works = err == nil
		try++
	}
	if !works {
		log.Fatalf("mealie connection cannot be established")
	}

	// API.
	startAPIFn, serverShutdown := setUpAPI(
		cfg.listenInterface,
		time.Duration(cfg.timeoutSecs)*time.Second,
		mealie.getRecipes,
		[]responseGenerator{
			&markdownGenerator{url: cfg.mealieBaseURL},
			&epubGenerator{url: cfg.mealieBaseURL},
			&pdfGenerator{url: cfg.mealieBaseURL},
			&htmlGenerator{url: cfg.mealieBaseURL},
		},
	)

	// Use default timeout for now.
	quitHook := func() error {
		return serverShutdown(0)
	}

	// Allow killing via signals, too. Listen for SIGINT (sent by user) and SIGTERM (sent by OS).
	signalQuit := make(chan os.Signal, 2) //nolint:gomnd
	signal.Notify(signalQuit, os.Interrupt, syscall.SIGTERM)
	go func() {
		for done := false; !done; {
			// Block until the signal channel has been notified, then call the quit hook. If there
			// is an error calling the quit hook, do not exit but continue to listen for signals.
			sig := <-signalQuit
			log.Printf("caught signal %v", sig)
			if err := quitHook(); err != nil {
				log.Printf("error shutting down due to signal: %s", err.Error())
			} else {
				done = true
				quit <- true
			}
		}
	}()

	// Actually start the API.
	startAPIFn()
	// Block until we are asked to quit.
	<-quit
}
