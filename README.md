## Export Recipes From Mealie To Various Formats And More

<!-- vim-markdown-toc GFM -->

* [About](#about)
* [Motivation](#motivation)
* [Supported Features](#supported-features)
* [Non-Features](#non-features)
* [Caveats](#caveats)
* [Working Principle](#working-principle)
* [How To Use](#how-to-use)
    * [Filtering And Examples](#filtering-and-examples)
* [How To Deploy](#how-to-deploy)
    * [Docker-Compose](#docker-compose)
    * [Systemd](#systemd)
    * [Manual](#manual)
* [Environment Variables](#environment-variables)
* [How To Contribute](#how-to-contribute)
* [Licence](#licence)

<!-- vim-markdown-toc -->

# About

This is `mealie-addons`, a stand-alone server that provides additional
functionality for the amazing [mealie] by utilising its REST API.
The main feature of `mealie-addons` is bulk export of recipies to various
formats such as PDF or EPUB to any device with a web browser.
The `mealie-addons` project is not affiliated with the [mealie] project.

# Motivation

The amazing [mealie] does not support bulk exports of recipies for offline
storage as of 2025-02-01 and version `v2.4.2`.
There is a [long standing issue] regarding document export, but the feature has
yet to be implemented.
This is the main reason that `mealie-addons` was devised.
Since it is a stand-alone service that runs alongside an existing [mealie]
instance, it was easy and fast to develop and to deploy.
If upstream [mealie] ever supports bulk recipe exports, `mealie-addons` will be
deprecated.

# Supported Features

- Export recipes to different formats for offline use.
  Currently supported are PDF, EPUB, HTML, and markdown.
- Trigger exports from any device with a web browser, be it computer, phone, or
  something else entirely.
- Use arbitrary filter queries to retrieve only those recipes that are relevant.
- Render markdown formatting in recipes for all output formats.
- Add all supported functionality to any [mealie] instance without the need for
  admin access.

# Non-Features

The following features are out of scope for `mealie-addons`:

- Encrypted connections:
  All connections to `mealie-addons` use unencrypted HTTP.
  An additional [nginx] deployment can provide an upgrade to encrypted HTTPS.
  In general, it is recommended to deploy `mealie-addons` behind a [VPN].
- Authentication:
  Anyone with network access to `mealie-addons` can retrieve the recipes
  accessible to it.
  An additional [oauth2-proxy] deployment can be used to limit access to a
  `mealie-addons` instance.
  In general, it is recommended to deploy `mealie-addons` behind a [VPN].

The following features are not implemented at the moment but can be considered
in scope for `mealie-addons`:

- Web interface:
  At the moment, `mealie-addons` is a pure backend service without a frontend.
  A frontend that simplifies the creation of a query and the selection of a file
  type would be beneficial.
- Pretty documents:
  The documents produced by `mealie-addons` are meant to be functional and not
  pretty.
  That is, they are supposed to be small, contain many internal and external
  links for ease of use, and be usable on memory-constrained devices.
  Nevertheless, improvements to the formatting of the generated documents, in
  particular PDFs, would be beneficial.

# Caveats

- Due to the way the markdown document is constructed, line breaks are not
  preserved in descriptions, steps, or ingredients.
- Since `mealie-addons` is a stand-alone server, it has to be hosted somewhere.
  In most cases, it will be hosted right next to an existing [mealie] instance.
  However, some cases will require hosting it separately.
- Since document conversion uses [pandoc], formats that are not supported by
  [pandoc] are hard to add.
  Furthermore, any bug in [pandoc] also affects `mealie-addons`.
  One such bug is that links within the document are broken for direct
  conversion from markdown to anything other than HTML.
  To work around this issue, `mealie-addons` first converts to HTML and then to
  the user's chosen output format.
- Since `mealie-addons` uses [mealie]'s REST API, it can only ever export data
  associated with a single group per instance.

# Working Principle

To integrate easily with any [mealie] instance, `mealie-exports` uses
[mealie's REST API] to retrieve data.
Based on a user's query, `mealie-exports` will retrieve all matching recipes
from the configured [mealie] instance.
Once retrieved, each recipe will be converted to markdown in memory.
Then, all recipes will be aggregated into a single markdown document in memory
along with a recipe index, a tag index, and a category index.
That document will then be converted to the user's chosen format using the
amazing [pandoc] and served as a file download.

# How To Use

The following examples assume an instance of `mealie-addons` accessible at
`http://mealie-addons`.
Replace that URL with your own.
A document download can be triggered by accssing a filetype-specific endpoint
with a browser.
Those endpoints are:

- EPUB:
  `http://mealie-addons/book/epub`
- PDF:
  `http://mealie-addons/book/pdf`
- HTML:
  `http://mealie-addons/book/html`
- markdown:
  `http://mealie-addons/book/markdown`

Each URL can be followed by query parameters to modify which recipes are
retrieved and in which order.
See [below](#filtering-and-examples) for more details.

## Filtering And Examples

Often, it is desirable to retrieve only a subset of all recipies stored in a
[mealie] instance.
To support this, `mealie-addons` will forward all query parameters to [mealie]'s
`/get/recipes` endpoint as is.
Hence, `mealie-export` supports all of [mealie]'s comprehensive [filtering]
features but not more.
Note that all query values have to use their [URL encoding].

For the following examples, it is assumed that your `mealie-addons` server can
be reached via `http://mealie-addons`.
Replace that URL with your own.

- Order recipes by name in ascending order and export to EPUB:
  `http://mealie-addons/book/epub?orderBy=name&orderDirection=asc`
- Get all recipes created after a certain date, in this example 2023-02-25, and
  export to PDF:
  `http://mealie-addons/book/pdf?queryFilter=recipe.createdAt%20%3E%3D%20%222023-02-25%22`
  Note that the value following the `queryFilter` query parameter is the
  [URL encoding] of the string `recipe.createdAt >= "2023-02-25"`.

# How To Deploy

In most cases, `mealie-addons` will be deployed next to an existing [mealie]
instance.

## Docker-Compose

This is the preferred way to deploy `mealie-addons`.
The following is a docker-compose file based on the [SQLite example] from the
official [mealie] documentation.
Simply add `mealie-addons` as a separate service as shown below.
In this example, `mealie-addons` will be accessible under the same URL as
[mealie] but on port 9926 as compared to [mealie]'s port 9925.
In this example, the given [API token] provides access to the recipes of a group
called `home`.
The meaning of each of the the [environment variables] is explained
[below](#environment-variables).

```yaml
---
services:
    mealie:
        image: ghcr.io/mealie-recipes/mealie:v2.4.2
        container_name: mealie
        restart: always
        ports:
            - "9925:9000"
        deploy:
            resources:
                limits:
                    memory: 1000M
        volumes:
            - mealie-data:/app/data/
        environment:
            ALLOW_SIGNUP: "false"
            PUID: 1000
            PGID: 1000
            TZ: America/Anchorage
            BASE_URL: https://mealie.yourdomain.com

    mealie-addons:
        image: mealie-addons:local
        container_name: mealie-addons
        restart: always
        ports:
            - "9926:9000"
        build:
            context: .
            dockerfile_inline: |
                FROM golang:1.23-bookworm AS builder
                RUN \
                  apt-get update && \
                  DEBIAN_FRONTEND=noninteractive apt-get install -y make git
                WORKDIR /app
                RUN \
                  git clone https://github.com/razziel89/mealie-addons . && \
                  make build

                FROM ubuntu:latest
                RUN \
                  apt-get update && \
                  DEBIAN_FRONTEND=noninteractive apt-get install -y \
                    pandoc texlive-latex-base texlive-latex-extra texlive-xetex && \
                  rm -rf /var/lib/apt/lists/*
                WORKDIR /app
                COPY --from=builder /app/mealie-addons .
                ENTRYPOINT ["/app/mealie-addons"]
        environment:
            MA_LISTEN_INTERFACE: ":9000"
            MA_RETRIEVAL_LIMIT: "5"
            MA_TIMEOUT_SECS: "60"
            MA_STARTUP_GRACE_SECS: "30"
            MEALIE_BASE_URL: "https://mealie.yourdomain.com/g/home"
            MEALIE_RETRIEVAL_URL: "http://mealie:9000"
            MEALIE_TOKEN: "/run/secrets/MEALIE_TOKEN"
            GIN_MODE: release
        secrets:
            - MEALIE_TOKEN

volumes:
    mealie-data:

secrets:
    MEALIE_TOKEN:
        # This is the file containing the token used to access mealie. The path
        # is relative to the location of this file.
        file: mealie_token.txt
```

You can then start `mealie-addons` like this by first building the required
docker image locally and then starting everything up:

```bash
docker compose build
docker compose up
```

## Systemd

Go to the project's release page to download the [latest release], select the
correct distribution for your system, and download it.
Then, add a file `/etc/systemd/system/mealie-addons.service` with the following
content.
The meaning of each of the the [environment variables] is explained
[below](#environment-variables).

```
[Service]
# Environment variables used to configure mealie-addons.
# Replace each <TODO> by an appropriate value.
Environment=MEALIE_BASE_URL=<TODO>/g/<TODO>
Environment=MEALIE_RETRIEVAL_URL=<TODO>
Environment=MEALIE_TOKEN=<TODO>
Environment=MA_LISTEN_INTERFACE=<TODO>:<TODO>
Environment=MA_RETRIEVAL_LIMIT=<TODO>
Environment=MA_STARTUP_GRACE_SECS=<TODO>
Environment=MA_TIMEOUT_SECS=<TODO>

# A local user account that this service shall run as. Do not use root.
User=<TODO>
# The directory where mealie-addons is located.
WorkingDirectory=<TODO>
# Replace <TODO> by the directory where mealie-addons is located.
ExecStart=<TODO>/mealie-addons

Environment=GIN_MODE=release
Restart=on-failure
Type=simple

[Unit]
Description=Mealie Addons

[Install]
WantedBy=multi-user.target
```

To launch the new service and make sure it is launched at system startup,
execute the following commands:

```bash
sudo systemctl daemon-reload
sudo systemctl enable mealie-addons
sudo systemctl start mealie-addons
```

## Manual

Go to the project's release page to download the [latest release], select the
correct distribution for your system, and download it.
Extract the downloaded archive and move the extracted binary to a location that
is in your `$PATH` such as `/usr/local/bin`.
Moving it there will likely require root permissions, e.g. via `sudo`.
Set all required [environment variables](#environment-variables) as explained
below and execute `mealie-addons` in your terminal.

# Environment Variables

The configuration of `mealie-addons` is done via [environment variables].
The following explains all [environment variables] understood by
`mealie-addons`.

- `MEALIE_BASE_URL`:
  The same value as the `BASE_URL` in your mealie config followed by `/g/` and
  by the name of the group whose data you wish to export.
  The first part is the URL that you can reach mealie from externally.

  - Example of a [mealie] instance at `http://my-mealie.org` and a group `home`:
    `http://my-mealie.org/g/home`

- `MEALIE_RETRIEVAL_URL` The URL that `mealie-addons` shall use to retrieve data
  from [mealie].
  This shall be identical to the `MEALIE_BASE_URL` if both services are not
  running on the same system.

  - Example of both running on the same system:
    `http://localhost:8013`

- `MEALIE_TOKEN`:
  An [API token] that can be used to access [mealie].
  Access to recipes will be restricted to whatever this token gives access to.
  This can also be a path to a file that contains the token.

- `MA_LISTEN_INTERFACE`:
  The network interface where `mealie-addons` shall be reachable in the format
  `interface:port`.
  Leave the interface part empty if you wish to listen on all interfaces.

  - Example listening on all network interfaces and port 8014:
    `:8014`
  - Example listening on the local loopback interface and port 8015:
    `127.0.0.1:8015`

- `MA_RETRIEVAL_LIMIT`:
  The number of concurrent connections `mealie-addons` shall use to [mealie]
  when retrieving recipe details.
  Do not make this a lot larger than 5.
  Depending on the performance of the server hosting mealie, this might have to
  be 2 or even 1 in order not to overburden the server with requests.

- `MA_STARTUP_GRACE_SECS`:
  The number of seconds that `mealie-addons` will attempt to connect to [mealie]
  at startup.
  This should be as short as possible but long enough for the [mealie] server to
  start up fully and accept connections.
  This configuration is important mostly if both services run on the same
  machine.

- `MA_TIMEOUT_SECS`:
  The number of seconds that `mealie-addons` may take at most to generate a file
  for download.
  This value helps prevent resource starvation on the server side due to very
  large numbers of recipes being retrieved.
  It also helps prevent running into HTTP timeouts.
  This value must be large enough for the file to be successfully generated and
  downloaded.

# How To Contribute

If you have found a bug and want to fix it, please simply go ahead and fork the
repository, fix the bug, and open a pull request to this repository!
Bug fixes are always welcome.

In all other cases, please open an issue on GitHub first to discuss the
contribution.
The feature you would like to introduce might already be in development.

# Licence

[GPLv3]

If you want to use this piece of software under a different, more permissive
open-source licence, please contact me.
I am very open to discussing this point.

[API token]: https://docs.mealie.io/documentation/getting-started/api-usage/#getting-a-token
[environment variables]: https://en.wikipedia.org/wiki/Environment_variable
[filtering]: https://docs.mealie.io/documentation/getting-started/api-usage/#filtering
[GPLv3]: ./LICENCE
[latest release]: https://github.com/razziel89/mdslw/releases/latest
[long standing issue]: https://github.com/mealie-recipes/mealie/issues/1306
[mealie's REST API]: https://docs.mealie.io/documentation/getting-started/api-usage/
[mealie]: https://mealie.io/
[nginx]: https://nginx.org/en/
[oauth2-proxy]: https://github.com/oauth2-proxy/oauth2-proxy
[pandoc]: https://pandoc.org/
[SQLite example]: https://docs.mealie.io/documentation/getting-started/installation/sqlite/
[URL encoding]: https://en.wikipedia.org/wiki/Percent-encoding
[VPN]: https://en.wikipedia.org/wiki/Virtual_private_network
