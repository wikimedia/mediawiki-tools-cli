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

### Service targets

Service commands can also be selected with an `@target` prefix. For example,
`mw @gerr<TAB>` completes to `@gerrit.wikimedia.org`; after accepting the
target, the next completion offers Gerrit commands. Short targets such as
`@phab` work too. Known MediaWiki hosts can be used directly, for example
`mw @en.wikipedia.org page list`. The explicit `@en.wiki` shortcut selects
English Wikipedia too. Global flags can precede the target, for example
`mw -v @en.wiki page list`. Use `mw help @en.wiki page` for command help.

Wikimedia wiki targets combine content commands with project analytics and events:

```sh
mw @en.wikipedia.org pageviews 'Main Page' --agent user
mw @commons.wikimedia.org top-pages
mw @en.wikipedia.org events tail --limit 10
```

Tab completion after the target offers all these capabilities. Analytics use the
selected hostname as their default `--project`; events filter by that hostname
using `--domain` (including streams other than recent changes). Explicit flags
can override those defaults. Content commands use the target's Action API URL.
Other MediaWiki hosts retain content commands only. The usual `mw wiki`,
`mw analytics` and `mw events` commands remain available.

Targets select a service command group; they do not claim that every command
connects to the URL typed. Cloud VPS commands use the OpenStack API endpoint
(`openstack.eqiad1.wikimediacloud.org`) while Horizon is its web interface
(`horizon.wikimedia.org`). Toolforge commands use SSH at
`login.toolforge.org`; a target such as `@mytool.toolforge.org` selects that
tool for supported `exec` and `cp` commands. The older
`tools.wmflabs.org/<tool>` web URL is a legacy web-service address, not the
Toolforge SSH endpoint.

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
