FROM golang:1.26-bookworm AS builder
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y make
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY *.go Makefile ./
RUN make build

FROM ubuntu:24.04 AS downloader_base
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y wget curl

FROM downloader_base AS downloader_pandoc
RUN wget -O /pandoc.deb https://github.com/jgm/pandoc/releases/download/3.9/pandoc-3.9-1-amd64.deb

FROM downloader_base AS downloader_fonts
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    unzip findutils rename gawk grep
RUN wget -qO /fonts.zip https://github.com/google/fonts/archive/main.zip
RUN unzip /fonts.zip $(\
  unzip -l /fonts.zip \
    | grep -E "fonts-main/ofl/noto(serif|sans)[^/]*/$" \
    | awk '{print $NF "*"}' \
)
RUN for font in $(find /fonts-main/ -iname "*.ttf"); do \
  new=$(basename "${font}" | tr -d '],[' | tr -d -- '-') && \
  mv "${font}" "$(dirname "${font}")/${new}"; \
done
WORKDIR /fonts
RUN find \
  /fonts-main/ofl/notoserif/ \
  /fonts-main/ofl/notoserifsc/ \
  /fonts-main/ofl/notoserifjp/ \
  /fonts-main/ofl/notoserifkr/ \
  /fonts-main/ofl/notosanssymbols/ \
  /fonts-main/ofl/notosanssymbols2/ \
  /fonts-main/ofl/notocoloremoji/ \
  /fonts-main/ofl/notoemoji/ \
  \
  -iname "*.ttf" | xargs cp -t /fonts
RUN cp /fonts-main/ofl/notoserif/NotoSerifwdthwght.ttf /fonts/main.ttf

FROM ubuntu:24.04
LABEL org.opencontainers.image.source=https://github.com/razziel89/mealie-addons
LABEL org.opencontainers.image.description="Export recipes from mealie to various formats and more."
LABEL org.opencontainers.image.licenses=GPLv3,OFL,others
COPY --from=downloader_pandoc /pandoc.deb .
RUN \
  apt-get update && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ./pandoc.deb \
    ca-certificates \
    texlive-latex-base \
    texlive-latex-extra \
    texlive-xetex \
    texlive-luatex \
  && \
  rm pandoc.deb && \
  rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=downloader_fonts ./fonts/*.ttf .
COPY --from=builder /app/mealie-addons .
ENTRYPOINT ["/app/mealie-addons"]
