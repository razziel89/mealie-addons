SHELL := /bin/bash -euo pipefail

CGO_ENABLED ?= 0
export CGO_ENABLED

default: lint

.PHONY: setup
setup:
	go mod download
	go mod tidy

.PHONY: update-deps
update-deps:
	go get -t -u
	$(MAKE) setup

.PHONY: build
build: mealie-addons

.PHONY: build-cross-platform
build-cross-platform:
	CLIVERSION=local goreleaser build --clean --snapshot

mealie-addons: *.go go.*
	go build -o mealie-addons ./...

.PHONY: lint
lint:
	golangci-lint run .
	mdslw --mode=check --report=changed .

test: .test.log

.test.log: go.* *.go
	go test | tee .test.log

coverage.html: go.* *.go
	go test -covermode=count -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

.PHONY: coverage
coverage: coverage.html
	xdg-open coverage.html
