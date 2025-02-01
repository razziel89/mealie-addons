## Export recipes from mealie to various formats and more

<!-- vim-markdown-toc GFM -->

* [About](#about)
* [Motivation](#motivation)
* [Working Principle](#working-principle)
* [Supported Features](#supported-features)
* [Caveats](#caveats)
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
functionality for the amazing [mealie] by using its REST API.
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

# Working Principle

To integrate easily with any [mealie] instance, `mealie-exports` uses
[mealie's REST API] to retrieve data.
Based on a user's query, `mealie-exports` will retrieve all matching recipes
from the configured [mealie] instance.
Once retrieved, each recipe will be converted to markdown.
Then, all recipes will be aggregated into a single markdown document along with
a recipe index, a tag index, and a category index.
That document will then be converted to the user's chosen format using the
amazing [pandoc].

# Supported Features

- Export recipes to different formats for offline use.
  Currently supported are PDF, EPUB, HTML, and markdown.
- Trigger exports from any device with a web browser, be it computer, phone, or
  something else entirely.
- Use arbitrary filter queries to retrieve only those recipes that are relevant.
- Render markdown formatting in recipes for all output formats.
- Add all supported functionality to any [mealie] instance without the need for
  admin access.

# Caveats

- Since document conversion uses [pandoc], formats that are not supported by
  [pandoc] are hard to add.
  Furthermore, any bug in [pandoc] also affects `mealie-addons`.
  One such bug is that links within the document are broken for direct
  conversion from markdown to anything other than HTML.
  To work around this issue, `mealie-addons` first converts to HTML and then to
  the user's chosen output format.
- Due to the way the markdown document is constructed, line breaks are not
  preserved in descriptions, steps, or ingredients.
- Since `mealie-addons` is a stand-alone server, it has to be hosted somewhere.
  In most cases, it will be hosted right next to an existing [mealie] instance.
  However, some cases will require hosting it separately.

# How To Deploy

In most cases, `mealie-addons` will be deployed next to an existing [mealie]
instance.

## Docker-Compose

This is the preferred way to deploy `mealie-addons`.

## Systemd

## Manual

Go to the project's release page to download the [latest release], select the
correct distribution for your system, and download it.
Extract the downloaded archive and move the extracted binary to a location that
is in your `$PATH` such as `/usr/local/bin`.
Moving it there will likely require root permissions, e.g. via `sudo`.

# Environment Variables

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

[GPLv3]: ./LICENCE
[latest release]: https://github.com/razziel89/mdslw/releases/latest
[long standing issue]: https://github.com/mealie-recipes/mealie/issues/1306
[mealie's REST API]: https://docs.mealie.io/documentation/getting-started/api-usage/
[mealie]: https://mealie.io/
[pandoc]: https://pandoc.org/
