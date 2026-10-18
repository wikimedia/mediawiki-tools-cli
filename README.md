# a Wikimedia CLI

A command-line interface for MediaWiki and Wikimedia developers, currently installed as `mw`. It provides a set of tools to simplify common tasks, such as managing a local development environment, interacting with GitLab and Gerrit, and more.

## Key Features

- **MediaWiki Development Environment**: A Docker-powered local environment for MediaWiki development.
- **GitLab & Gerrit Integration**: Tools for managing merge requests, patches, and repository interaction.
- **ToolHub & CodeSearch**: Quick access to Wikimedia developer tools and codebase search.
- **Update System**: Easy updates to the latest version of the CLI.

## Getting Started

### Usage

For user-facing documentation and a guide on how to use `mw`, visit the [MediaWiki.org page](https://www.mediawiki.org/wiki/Cli).

### Analytics and REST APIs

`mw analytics` queries the [Wikimedia Analytics API](https://doc.wikimedia.org/generated-data-platform/aqs/analytics-api/) for pageviews, edits, editors, new pages, unique devices, and media requests. Omit the article from `pageviews` or `edits` for project totals. Rankings support `--date YYYYMMDD`, `--month YYYYMM`, and `--limit`; `--offset` skips leading results before applying the limit. Without a date, they find the latest available day within the past week. Tables are the default in a terminal; pipes receive JSON. Use `--output json|table|jq|csv|graph` to choose explicitly. Graphs show time series as lines and rankings as horizontal bars; `--graph-width` overrides the detected terminal width. Page names in ranking tables and graphs are clickable in terminals that support hyperlinks.

```sh
mw analytics pageviews 'Albert Einstein' --agent user --output graph
mw analytics top-pages --project en.wikipedia.org --date 20250101 --offset 1 --limit 10 --output graph
mw analytics mediarequests top --month 202501
mw api 'metrics/pageviews/top/en.wikipedia.org/all-access/2025/01/01' --output jq --format '.items[0].articles[:5]'
```

`mw api` accepts REST endpoints relative to `https://wikimedia.org/api/rest_v1/`, an alternative `--base-url`, or an absolute URL. Use `--method` (`-X`), repeatable `--header` (`-H`) and `--field` (`-f`) flags, or `--input FILE` (`-` for stdin). Fields become a JSON POST body by default, or query parameters with `--method GET`. `--include` prints response headers; `--raw` preserves the response body.

### Installation

Follow the [installation guide](https://www.mediawiki.org/wiki/Cli/guide/Installation) on MediaWiki.org.

## Contributing & Development

We welcome contributions! Please see the following documents for more information:

- **[CONTRIBUTING.md](CONTRIBUTING.md)**: Guidelines for contributing to the project and the workflow we follow.
- **[DEVELOPMENT.md](DEVELOPMENT.md)**: Detailed instructions on how to set up your local development environment, build the project, and run tests.

### Executable names

`mw` remains the default. The same executable can be renamed or symlinked as `wm` (or another name); help, examples, and generated shell completion follow its invocation name. Both names use the existing `mwcli` configuration and credentials. Build another name with `make BINARY_NAME=wm build`, then run `./bin/wm`.

### Using a development build

The checkout can live anywhere. In Bash, build the CLI and select it for the current shell:

```sh
eval "$(make -s dev)"
command -v mw
```

From another directory, replace `/path/to/cli` with your checkout location:

```sh
eval "$(make -s -C "/path/to/cli" dev)"
```

For convenience, add an alias to your Bash configuration, using your checkout's absolute path:

```sh
alias mw-dev='eval "$(make -s -C "/path/to/cli" dev)"'
```

Run `mw-dev` to build and activate it. Repeating activation avoids duplicate PATH entries. This affects only the current shell; rerun after changing code or branches. You can also use `make build` and run `./bin/mw` directly.

Existing Bash completion follows the selected binary. To load it explicitly, run `source <(mw completion bash)`.

## Support

- **Phabricator**: [#mwcli](https://phabricator.wikimedia.org/project/view/5331/)
- **IRC**: `#mediawiki` on [Libera.Chat](https://libera.chat/)
- **Mailing List**: [wikitech-l](https://lists.wikimedia.org/postorius/lists/wikitech-l.lists.wikimedia.org/)

## CI & Integration tests

This repository has continuous integration setup on GitLab. You can read more in [CI.md](CI.md).

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
