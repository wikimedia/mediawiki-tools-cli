# Development Guide

This document provides detailed information for developers who want to contribute to the `mwcli` project.

## Environment Setup

### Requirements

- [Go](https://go.dev/) 1.26+ installed (matching the version in `go.mod`).
- [bingo](https://github.com/bwplotka/bingo) tool (used for managing Go-based build tools).

### Initial Setup

1.  **Clone the repository**:
    It is recommended to clone this repository to your `$GOPATH` (usually `~/go`), although modern Go versions support development outside of `$GOPATH`.
    ```sh
    mkdir -p ~/go/src/gitlab.wikimedia.org/repos/releng
    cd ~/go/src/gitlab.wikimedia.org/repos/releng
    git clone https://gitlab.wikimedia.org/repos/releng/cli.git
    cd cli
    ```

2.  **Install `bingo`**:
    ```sh
    go install github.com/bwplotka/bingo@v0.9.0
    ```

3.  **Install project tools**:
    Use `bingo` to install all the tools required for development (linting, code generation, etc.).
    ```sh
    bingo get
    ```

## Building

To build the `mw` binary:

```sh
make build
```

The resulting binary will be located at `./bin/mw`.

### Using the MediaWiki development environment with external hostnames

The Docker development environment normally selects a wiki database from the first label in the request hostname. To serve the default wiki at an additional hostname, add that hostname to both comma-separated context variables:

- `MEDIAWIKI_VIRTUAL_HOSTS` controls which hostnames the nginx proxy accepts. It defaults to `*.mediawiki.local.wmftest.net`.
- `MEDIAWIKI_DEFAULT_HOSTS` maps exact hostnames to the configured default wiki database (`default` unless changed).
- `MEDIAWIKI_DEFAULT_HTTPS_HOSTS` marks exact hostnames whose public TLS proxy should be used as the wiki's canonical HTTPS URL, preventing redirects to the internal development port.

For example, set both variables in the context's `.env` file and recreate the MediaWiki service. This keeps the normal per-wiki hostname behavior while allowing selected external names to serve the default wiki.

### Optional Traefik path gateway

mwcli also includes an optional Traefik service for path-based access without replacing mwdd's nginx proxy. Start it with `mw docker traefik create`; it binds to `127.0.0.1:8081` by default. The dashboard is available at `/proxy/dashboard/`, wiki databases use mwdd's hostname convention at `/proxy/<database>/` (for example, `/proxy/default/`), and web services with Traefik Compose labels are discovered as they start. Their route is `/proxy/<service>/` by default. The Compose labels use the same context variables as `VIRTUAL_HOST` and `VIRTUAL_PORT`; set `<SERVICE>_PROXY_PATH` to customize a route prefix. Stop it with `mw docker traefik stop` or remove it with `mw docker traefik destroy`.

Gateway authentication is enabled by default. The first `create` generates a random password for the `mwcli` user, displays it once, and saves it with owner-only permissions under the context's Traefik auth directory. Show it again with `mw docker traefik credentials`. To disable gateway auth for a context, set `TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file` in its `.env` before creating the gateway and recreating the web services. The Traefik Docker provider uses a private, GET-only Docker API proxy; it does not publish the API port.

To expose the gateway from a development machine that is not directly reachable,
start an optional tunnel. Cloudflare remains the default:

```sh
mw docker tunnel start
mw docker tunnel start --provider holesail
```

Select `cloudflare` or `holesail` with `--provider`, or set the context default
with `mw docker env set TUNNEL_PROVIDER holesail`. A successful `start` remembers
the provider and stops any running tunnel using the other provider. If startup
fails, the previous tunnel remains running. `mw docker tunnel status`, `url`, and
`logs --follow` use the saved provider; `--provider` overrides it for a command.
`mw docker tunnel stop` stops both providers while leaving the gateway and wiki
running. `stop --provider holesail` stops only Holesail. Gateway authentication
remains enabled by default with either provider.

Cloudflare prints a temporary public `trycloudflare.com` HTTPS URL that works in
an ordinary browser. Quick Tunnels do not guarantee uptime, have a 200 in-flight
request limit, and do not support Server-Sent Events (SSE).

Holesail prints a private `hs://s000...` connection string. Treat it as a secret.
The optional container is built on first use with Holesail 2.4.4 and locked npm
dependencies; nothing needs installing on the server host. On the connecting
computer, install Holesail and run the client command printed by `start`, such as:

```sh
npm install --global holesail@2.4.4
holesail 'hs://s000...' --host 127.0.0.1 --port 8082
```

Then open `http://127.0.0.1:8082/proxy/dashboard/` or
`http://127.0.0.1:8082/proxy/default/wiki/Main_Page`, using the gateway login from
`mw docker traefik credentials`. The local port can be changed. Holesail does
not provide a public browser URL: every connecting computer needs a client.
Connections require working peer discovery and peer connectivity on the network.
Server restarts generate a new connection string; use `mw docker tunnel url` to
retrieve it. HTTP links use the local client origin; Cloudflare and other HTTPS
proxies retain their forwarded HTTPS origin.

For a manual end-to-end Holesail check on a machine with an installed default
wiki, default gateway authentication enabled and no active tunnel, run
`./tests/test-docker-tunnel-holesail.sh`. It uses a separate client container
with Linux host networking on the same VM and checks gateway authentication,
the dashboard, wiki pages and the canonical URL. Set `MW_BIN`, `TUNNEL_CONTEXT`,
or `CLIENT_PORT` to override its defaults. It stops its test tunnel and client,
and restores the previous provider setting. Real public DHT connectivity is
tested explicitly here; the normal CI suite uses offline lifecycle tests.

The gateway has a separate loopback-only port for local testing, while mwdd's existing nginx proxy remains the public entry point. A host-specific nginx route forwards `/proxy/` to Traefik internally without exposing Traefik's port directly. mwdd's normal host-based wiki URLs continue to use nginx. Some tools require app-specific base-path settings for all links and assets to work under a prefix; prefix routing alone does not guarantee that every tool's UI is fully compatible.

### Running the development build

Follow the [development build instructions in README.md](README.md#using-a-development-build)
to put `bin/mw` first on your PATH. This lets you copy documentation examples
using `mw` without changing the command name. You can also run `./bin/mw` directly
from the repository root.

### Makefile commands

Many Makefile commands exist that you might find useful:

- `make build`: Builds a new binary.
- `make release`: Builds multiple release binaries to `_release`.
- `make test`: Run unit tests.
- `make recipe-validate`: Validate all checked-in `mount/dev/recipes/*.yml` files.
- `make lint`: Run basic go linting.
- `make linti`: Run custom mwcli command linting (lint internal).
- `make fix`: Run basic lint fixes.
- `make vet`: Run `go vet`.
- `make generate`: Run code generation tools.
- `make docs`: Generate documentation.

## Project Structure

The project is organized into several key directories:

- `cmd/`: Creation and execution of the top level Cobra command.
- `commands/`: Implementation of various CLI commands, grouped by sub-system.
- `internal/`: Private library code.
    - `internal/cli/`: High level things used across the CLI.
    - `internal/cmd/`: Packages for commands that make up part of the CLI, binding to cobra.
    - `internal/cmdgloss/`: Glossy output for users.
    - `internal/codesearch/`: Client for interacting with codesearch.wikimedia.org.
    - `internal/config/`: CLI wide configuration.
    - `internal/eventlogging`: Client to submit events to Wikimedia Event Logging.
    - `internal/exec`: Wrapper around `exec` for easily running commands and capturing output.
    - `internal/gitlab`: Client for interacting with gitlab.wikimedia.org.
    - `internal/mediawiki`: Interact with a MediaWiki installation directory on disk.
    - `internal/mwdd`: Package for the docker-compose powered development environment.
    - `internal/toolhub`: Client for interacting with toolhub.wikimedia.org.
    - `internal/updater`: Code for updating the CLI.
    - `internal/util`: DEPRECATED: Independent packages (slowly being moved to `pkg/`).
- `pkg/`: Independently useful packages that do not bind to mwcli code or concepts.
- `mount/`: Files that are mounted or embedded into the environment (e.g., dev recipes).
- `tests/`: Integration tests that are run as part of CI.
- `tools/`: Various tools to make working with this repository easier (code-gen, docs-gen, etc.).

## Common Development Tasks

### Adding a New Command

1.  Create a new directory or file under `commands/`.
2.  Define your `cobra.Command`.
3.  Register the command in `cmd/root.go` by adding it to the `AddCommand` list.

### Running Tests

- **Unit Tests**:
  ```sh
  make test
  ```
- **Integration Tests**:
  Located in the `tests/` directory.
  ```sh
  ./tests/test-general.sh
  ```
  For the dev environment:
  ```sh
  ./tests/test-docker-general.sh
  ./tests/test-docker-get-code.sh
  # ... and others
  ```

### Dev recipe files (`mount/dev/recipes/*.yml`)

This repo includes sample/dev recipe files under `mount/dev/recipes/`.
They are embedded into the binary and extracted into your mwdd context under `recipes/`.

To validate recipes:
```sh
make recipe-validate
```

## Releasing

Releases are automatically built and published by GitLab CI after pushing a tag.
Tags should follow [semver](https://semver.org/) and release notes should be written prior to tagging.

### Review changes since the last tag

Before creating a release tag, review the commit history from the latest tag to your current checkout:

```sh
git log --oneline "$(git describe --tags --abbrev=0)"..HEAD
```

and or the diff..

```sh
git diff "$(git describe --tags --abbrev=0)"
```

This helps confirm exactly which commits will be included in the next release.

### Process

1.  Add release notes for the release into `CHANGELOG.md` under a new header (e.g., `## v0.2.1`).
2.  Tag the commit: `git tag vx.x.x` (the `v` prefix is required).
3.  Push the tag: `git push origin vx.x.x`.
4.  Watch the pipeline run on GitLab.
5.  Check that the release appears on the releases page.
6.  Update the version in the installation docs on mediawiki.org.

## Docs

Docs for [mediawiki.org](https://www.mediawiki.org/wiki/Cli) are automatically generated by CI when a release is made.
They can also be manually generated and pushed:

```sh
make docs
# To publish (requires credentials)
make user="someUser" password="somePassword" docs-publish
```
