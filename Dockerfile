FROM golang:1.23-bookworm AS builder
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y make git wget
RUN wget -O /pandoc.deb https://github.com/jgm/pandoc/releases/download/3.6.3/pandoc-3.6.3-1-amd64.deb
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
    texlive-xetex \
    texlive-luatex \
  && \
  rm -rf /var/lib/apt/lists/*
COPY --from=builder /pandoc.deb .
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y ./pandoc.deb \
  && \
  rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY ./fonts/*.ttf .
COPY --from=builder /app/mealie-addons .
ENTRYPOINT ["/app/mealie-addons"]
