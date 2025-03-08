// Package main contains the server code.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/html"
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

	{
		copyCfg := cfg
		copyCfg.mealieToken = "***"
		log.Printf("using config: %+v", copyCfg)
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

	htmlHooks := []func(*html.Node) (*html.Node, error){}
	switch cfg.imageAction {
	case "ignore": // No-op.
	case "remove":
		log.Println("image tags will be removed from resulting documents")
		hook := func(htmlInput *html.Node) (*html.Node, error) {
			return removeAllHtmlElements(htmlInput, "img")
		}
		htmlHooks = append(htmlHooks, hook)
	case "embed":
		log.Println("image tags will be embedded into resulting documents")
		retrievalEndpoint := fmt.Sprintf(
			"http://127.0.0.1:%d/media/",
			cfg.listenPort,
		)
		hook := func(htmlInput *html.Node) (*html.Node, error) {
			return redirectImgSources(htmlInput, "/api/media/recipes/", retrievalEndpoint)
		}
		htmlHooks = append(htmlHooks, hook)
	}

	updateAttrsHook := func(htmlInput *html.Node) (*html.Node, error) {
		return updateHtmlAttrs(htmlInput, cfg.htmlAttrsMod, cfg.htmlAttrsRm)
	}
	htmlHooks = append(htmlHooks, updateAttrsHook)

	pandoc := pandoc{options: cfg.pandocFlags, htmlHooks: htmlHooks}
	err = pandoc.loadFonts(cfg.pandocFontsDir)
	if err != nil {
		log.Printf("failed to load fonts, skipping: %s", err.Error())
	}

	// API.
	startAPIFn, serverShutdown := setUpAPI(
		cfg.listenInterface,
		time.Duration(cfg.timeoutSecs)*time.Second,
		mealie.getRecipes,
		mealie.getMedia,
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
