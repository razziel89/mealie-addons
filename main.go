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

	var limiter chan bool = nil
	if cfg.retrievalLimit > 0 {
		log.Printf("retrieving at most %d recipes in parallel", cfg.retrievalLimit)
		limiter = make(chan bool, cfg.retrievalLimit)
	}

	mealie := mealie{url: cfg.mealieRetrievalURL, token: cfg.mealieToken, limiter: limiter}
	works, try := false, 1
	var group string
	for !works && try <= cfg.startupGraceSecs {
		var err error
		group, err = mealie.check()
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

	cfg.mealieBaseURL = cfg.mealieBaseURL + "/g/" + group

	var htmlHook func([]byte) ([]byte, error)
	switch cfg.imageAction {
	case "ignore": // No-op.
	case "remove":
		log.Println("image tags will be removed from HTML")
		htmlHook = func(html []byte) ([]byte, error) {
			return removeAllHtmlElements(html, "img")
		}
	}

	pandoc := pandoc{options: cfg.pandocFlags, htmlHook: htmlHook}
	err = pandoc.loadFonts(cfg.pandocFontsDir)
	if err != nil {
		log.Printf("failed to load fonts, skipping: %s", err.Error())
	}

	// API.
	startAPIFn, serverShutdown := setUpAPI(
		cfg.listenInterface,
		time.Duration(cfg.timeoutSecs)*time.Second,
		mealie.getRecipes,
		[]responseGenerator{
			&markdownGenerator{url: cfg.mealieBaseURL, pandoc: &pandoc},
			&epubGenerator{url: cfg.mealieBaseURL, pandoc: &pandoc},
			&pdfGenerator{url: cfg.mealieBaseURL, pandoc: &pandoc},
			&htmlGenerator{url: cfg.mealieBaseURL, pandoc: &pandoc},
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
