# a Wikimedia CLI

Explore Wikimedia data, work with services, and develop MediaWiki from your terminal. The default command is `mw`.

## Key Features

- **Wikimedia data:** Query analytics APIs, graph results, and follow event streams.
- **Wikis:** Read and edit pages on MediaWiki wikis.
- **Cloud VPS:** Find instances, connect over SSH, and manage compute and storage.
- **Code and collaboration:** Work with GitLab and Gerrit, and search Wikimedia code.
- **Development environments:** Run MediaWiki and supporting services with `mw dev` and Docker Compose, on your machine or a remote host.
- **Updates:** Update the CLI from releases or CI builds.

## Getting Started

### Usage

Read the [guides in this repository](docs/index.md), or visit the [published documentation on MediaWiki.org](https://www.mediawiki.org/wiki/Cli).

### Analytics and REST APIs

`mw analytics` queries the [Wikimedia Analytics API](https://doc.wikimedia.org/generated-data-platform/aqs/analytics-api/) for pageviews, edits, editors, new pages, unique devices, and media requests. Omit the article from `pageviews` or `edits` for project totals. Rankings support `--date YYYYMMDD`, `--month YYYYMM`, and `--limit`; `--offset` skips leading results before applying the limit. Without a date, they find the latest available day within the past week. Tables are the default in a terminal; pipes receive JSON. Use `--output json|table|jq|csv|graph` to choose explicitly. Graphs show time series as lines and rankings as horizontal bars; `--graph-width` overrides the detected terminal width. Page names in ranking tables and graphs are clickable in terminals that support hyperlinks.

```sh
mw analytics pageviews 'Albert Einstein' --agent user --output graph
mw analytics top-pages --project en.wikipedia.org --date 20250101 --offset 1 --limit 10 --output graph
mw analytics mediarequests top --month 202501
mw api 'metrics/pageviews/top/en.wikipedia.org/all-access/2025/01/01' --output jq --format '.items[0].articles[:5]'
```

`mw api` accepts REST endpoints relative to `https://wikimedia.org/api/rest_v1/`, an alternative `--base-url`, or an absolute URL. Use `--method` (`-X`), repeatable `--header` (`-H`) and `--field` (`-f`) flags, or `--input FILE` (`-` for stdin). Fields become a JSON POST body by default, or query parameters with `--method GET`. `--include` prints response headers; `--raw` preserves the response body.

### Tool traffic statistics

Query the public [Toolviews API](https://toolviews.toolforge.org/api/) with `mw tools toolviews` (or `mw tools views`). Counts come from Toolforge webserver access logs and include automated traffic. Dates use `YYYY-MM-DD` and are inclusive; the default is the last 30 completed UTC days. Missing observations remain missing.

```sh
mw tools toolviews toolviews --output graph
mw tools toolviews toolviews --start 2026-10-01 --end 2026-10-07
mw tools toolviews --date 2026-10-27 --output table
mw tools toolviews list
```

Tables are the default in a terminal and JSON in pipes. Choose `--output json|table|jq|csv|graph`; use `--graph-width` to set graph width. Graphs show a tool's daily series, or a single-day ranking across tools. JSON and CSV contain `date`, `tool`, and `hits`, with `hits` representing webserver hit counts. No Toolforge login is required. Tool names for `list` and shell completion come from the API and share a disk cache under `$XDG_CACHE_HOME/mwcli/toolviews-tools.json` (normally `~/.cache/mwcli/toolviews-tools.json`). The first lookup fetches and caches names; subsequent lookups return locally. After 24 hours, cached names are returned immediately while a background process refreshes them. Failed refreshes retain the last good list and defer retries for one minute. Use `mw tools toolviews list --refresh` to force a synchronous update. With shell completion loaded (for Bash: `source <(mw completion bash)`), type `mw tools views ` and press Tab. Completion lookups have a two-second timeout.

Toolforge management commands now live under `mw toolforge` (for example, `mw toolforge job list --tool mytool`). `tools` and `toolforge` are separate command groups.

### Installation

Follow the [installation guide](https://www.mediawiki.org/wiki/Cli/guide/Installation) on MediaWiki.org.

## Contributing & Development

To contribute, see:

- **[CONTRIBUTING.md](CONTRIBUTING.md)**: Contribution workflow and review.
- **[DEVELOPMENT.md](DEVELOPMENT.md)**: Setup, builds, tests, and releases.

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

See [CI.md](CI.md) for GitLab checks, GitHub integration tests, and runner setup.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
