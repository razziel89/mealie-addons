FROM golang:1.23-bookworm AS builder
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y make git
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY *.go Makefile ./
RUN make build

FROM ubuntu:24.04
LABEL org.opencontainers.image.source=https://github.com/razziel89/mealie-addons
LABEL org.opencontainers.image.description="Export recipes from mealie to various formats and more."
LABEL org.opencontainers.image.licenses=GPLv3
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    pandoc \
    texlive-latex-base \
    texlive-latex-extra \
    texlive-xetex\
  && \
  rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/mealie-addons .
ENTRYPOINT ["/app/mealie-addons"]
