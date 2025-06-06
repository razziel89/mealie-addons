package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

const (
	defaultTimeout    = 2 * time.Second
	readHeaderTimeout = 5 * time.Second
)

type healthResponse struct {
	OK   bool   `json:"ok"`
	UUID string `json:"uuid"`
}

var instanceUUID = uuid.New().String()

type responseGenerator interface {
	commonName() string
	extension() string
	mimeType() string
	response(context.Context, []recipe, time.Time) ([]byte, error)
}

func timedOut(ctx context.Context, c *gin.Context, msg string) bool {
	select {
	case <-ctx.Done():
		err := ctx.Err()
		msg := fmt.Sprintf("timeout %s: %s", msg, err.Error())
		log.Println(msg)
		c.String(http.StatusInternalServerError, msg)
		return true
	default:
		return false
	}
}

func setUpAPI(
	iface string,
	timeout time.Duration,
	getRecipes getRecipesFn,
	getMedia getMediaFn,
	generators []responseGenerator,
) (func(), func(time.Duration) error) {
	router := gin.Default()

	for _, generator := range generators {
		generator := generator
		log.Println("setting up endpoint for", generator.commonName())
		router.GET("/book/"+generator.commonName(), func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
			defer cancel()

			now := time.Now()
			// Set headers that trigger the download dialogue in the browser.
			filename := fmt.Sprintf(
				"recipes-%s.%s",
				now.Format(time.RFC3339),
				generator.extension(),
			)
			c.Writer.Header().
				Set("Content-Disposition", "attachment; filename="+filename)
			c.Writer.Header().Set("Content-Type", generator.mimeType())

			if timedOut(ctx, c, "before getting recipes") {
				return
			}

			// TODO: merge with default query parameters taken from env var.
			recipes, err := getRecipes(ctx, c.Request.URL.Query())

			if timedOut(ctx, c, "while getting recipes") {
				return
			}

			if err == nil {
				log.Printf("retrieved %d recipes for %s", len(recipes), generator.mimeType())
			}

			// Generate the file that shall be downloaded.
			var response []byte
			if err == nil {
				response, err = generator.response(ctx, recipes, now)
			}

			if timedOut(ctx, c, "while generating the file") {
				return
			}

			if err == nil {
				c.Writer.Header().Set("Content-Length", fmt.Sprint(len(response)))

				// Pass the file along.
				var written int64
				written, err = io.Copy(c.Writer, bytes.NewReader(response))
				log.Printf("written %d bytes, expected %d bytes", written, len(response))
				if int(written) != len(response) && err == nil {
					err = fmt.Errorf("failed to download everything")
				}
			}

			if err == nil {
				msg := fmt.Sprintf("%s endpoint accessed successfully", generator.mimeType())
				log.Println(msg)
				c.Status(http.StatusOK)
			} else {
				msg := fmt.Sprintf("unexpected error %s", err.Error())
				log.Println(msg)
				c.String(http.StatusInternalServerError, msg)
			}
		})
	}

	log.Printf("setting up endpoint for media retrieval")
	router.GET("/media/:uuid/:what/:filename", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		uuid := c.Param("uuid")
		what := c.Param("what")
		filename := c.Param("filename")
		if strings.HasSuffix(filename, ".webp.jpeg") {
			filename = strings.TrimSuffix(filename, ".jpeg")
		}

		media, err := getMedia(ctx, uuid, filename, what)

		if media.mime == "image/webp" {
			log.Printf("converting webp to jpeg: %s/%s", uuid, filename)
			// LaTeX doesn't understand webp images. Thus, we have to decode them and re-encode
			// them.
			var image image.Image
			image, err = webp.Decode(bytes.NewReader(media.content))
			buf := bytes.Buffer{}
			if err == nil {
				err = jpeg.Encode(&buf, image, nil)
			}
			media.content = buf.Bytes()
			media.mime = "image/jpeg"
		}

		if err == nil {
			c.Writer.Header().Set("Content-Type", media.mime)
			_, err = io.Copy(c.Writer, bytes.NewReader(media.content))
		}
		if err == nil {
			c.Status(http.StatusOK)
		} else {
			msg := fmt.Sprintf("unexpected error %s", err.Error())
			log.Println(msg)
			c.String(http.StatusInternalServerError, msg)
		}
	})

	log.Printf("setting up health check endpoint")
	router.GET("/health", func(c *gin.Context) {
		status := healthResponse{OK: true, UUID: instanceUUID}
		c.JSON(http.StatusOK, status)
	})

	server := &http.Server{
		Addr:              iface,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	shutdownFn := func(timeout time.Duration) error {
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		log.Println("shutting down the webserver within", timeout)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return server.Shutdown(ctx)
	}

	runFn := func() {
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("%s", err.Error())
			}
		}()
	}

	return runFn, shutdownFn
}

func healthCheck(selfURL string) error {
	sleeptime := time.Second
	retries := 30
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Duration(retries)*sleeptime)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "GET", selfURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to build health check request: %s", err.Error())
	}

	success := false
	retry := 0
	var response *http.Response
	for !success {
		response, err = http.DefaultClient.Do(request)
		if err == nil {
			success = true
		} else if retry >= retries {
			return fmt.Errorf("failed to execute health check request: %s", err.Error())
		} else {
			retry++
			time.Sleep(sleeptime)
		}
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected reply for health check: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read body for health check: %s", err.Error())
	}

	var status healthResponse
	err = json.Unmarshal(body, &status)
	if err != nil {
		return fmt.Errorf(
			"failed to parse body for health check: %s, body: %s",
			err.Error(), string(body),
		)
	}

	if status.UUID == instanceUUID {
		log.Println("health check successful")
		return nil
	}
	return fmt.Errorf(
		"instance UUID mismatch in health check, want '%s' but got '%s'",
		instanceUUID, status.UUID,
	)
}
