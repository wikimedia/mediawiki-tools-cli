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

- `MEDIAWIKI_VIRTUAL_HOSTS` controls the wildcard hostnames registered in the local DNS proxy. It defaults to `*.mediawiki.local.wmftest.net`; Traefik routes unmatched hostnames to the default wiki.
- `MEDIAWIKI_DEFAULT_HOSTS` maps exact hostnames to the configured default wiki database (`default` unless changed).
- `MEDIAWIKI_DEFAULT_HTTPS_HOSTS` marks exact hostnames whose public TLS proxy should be used as the wiki's canonical HTTPS URL, preventing redirects to the internal development port.

For example, set both variables in the context's `.env` file and recreate the MediaWiki service. This keeps per-wiki hostname selection while allowing selected external names to serve the default wiki.

### Traefik proxy and path gateway

mwcli uses Traefik as its default web proxy. It replaces nginx and listens on the configured `PORT` (8080 by default), routing the existing local virtual hosts as well as `/proxy/<service>/` paths. MediaWiki keeps its hostname based wiki selection for `*.mediawiki.local.wmftest.net`; the path gateway maps `/proxy/<database>/...` to the matching wiki, and an unmatched hostname uses the default wiki. The dashboard is available at `/proxy/dashboard/`, and web services with Traefik Compose labels are discovered as they start. Set `<SERVICE>_PROXY_PATH` to customize a path route prefix. Authentication is enabled by default; the first Docker service create generates a password. Show it with `mw docker traefik credentials`. Stop the proxy with `mw docker traefik stop` or remove it with `mw docker traefik destroy`.

Proxy routes use a gateway login page and a signed session cookie that expires after 12 hours. The first Docker service `create` generates a random password for the `mwcli` user and saves it with owner-only permissions; show it with `mw docker traefik credentials`. The credentials command also prints single-use login links for localhost and any active tunnel, valid for five minutes; use `--url` to select a different gateway URL. Existing credentials are preserved on upgrade. Sign out through the dashboard or `/_mwcli/logout`. Application logins and Bearer tokens work independently of the gateway session. Scripts can still use HTTP Basic credentials, for example `curl -u mwcli:PASSWORD http://localhost:8080/proxy/dashboard/api/status`. To disable proxy authentication for a context, set `TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file` in its `.env` and recreate the web services. The login service and GET-only Docker API proxy are internal and publish no ports.

To expose the gateway from a development machine that is not directly reachable, start an optional Cloudflare Quick Tunnel with `mw docker tunnel start`. The command starts Traefik if needed and prints the temporary `trycloudflare.com` URL. Use `mw docker tunnel url` to print the current URL again, `mw docker tunnel status` to check it, `mw docker tunnel logs --follow` to follow its logs, and `mw docker tunnel stop` to close the public tunnel while leaving the gateway running. The gateway's generated login remains required. Quick Tunnels are temporary development links, do not guarantee uptime, have a 200 in-flight request limit, and do not support Server-Sent Events (SSE).

Traefik binds the existing `PORT` on all interfaces, so local virtual hosts and remote `/proxy/...` routes use the same listener. The root path on an unmatched host serves the default wiki behind the same authentication. Some tools require app-specific base-path settings for all links and assets to work under a prefix; prefix routing alone does not guarantee that every tool's UI is fully compatible.

### Cloud VPS sessions

Open a Cloud VPS shell without manually building a hostname or bastion command:

```sh
mw cloudvps ssh gitlab-runner-addshore-1018 --project mwcli --user addshore
mw cloudvps ssh --project mwcli  # Choose a VM from the public instance inventory
mw cloudvps connect --project mwcli --name gitlab-runner-addshore-1018
```

Save defaults for repeated use:

```sh
mw config set cloud_vps.default_project mwcli
mw config set cloud_vps.ssh.user addshore
mw cloudvps ssh gitlab-runner-addshore-1018
mw cloudvps ssh gitlab-runner-addshore-1018 -- uname -a
```

The command uses your local OpenSSH client and retains your SSH configuration,
keys, agent, and host-key checks. It supplies `bastion.wmcloud.org` when no SSH
proxy is configured. You still need project membership and a public key registered
at [Wikimedia IDM](https://idm.wikimedia.org). On an interactive first connection,
the command asks for and remembers your developer shell username if SSH is using
your local default username. `--no-interaction` also enables SSH batch mode;
preconfigure your username, keys and known hosts for unattended commands.

Short names and the interactive chooser do not require OpenStack application
credentials. `--id` uses the existing per-project credentials to resolve a VM's
name. The public chooser lists instances, not the instances you are authorized
to access. Use a full `*.wikimedia.cloud` hostname to bypass project selection,
or provide a VM name to bypass public discovery if the inventory is unavailable.

Use `--dry-run` to inspect the command, `-vv` for SSH diagnostics, `--tty` for a
remote command needing a terminal, and repeated `--ssh-option` flags for native
OpenSSH options. Remote arguments are literal; use `-- sh -lc '...'` for shell
expressions. SSH exit status is preserved for scripting. The optional
`cloud_vps.ssh.region` and `cloud_vps.ssh.jump_host` settings change the defaults;
`--jump-host none` disables the automatic bastion.

This builds on [T292906](https://phabricator.wikimedia.org/T292906), its
`addshore-aug-2025` SSH prototype, and the native SSH session pattern in
[Toolforge Gen CLI](https://gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/-/blob/main/cmd/shell.go).

The existing `mw cloudvps compute ssh <instance-id>` command remains available
for OpenStack-based connections and `--script` / `--sudo` runner provisioning.
Its `--jump` flag and script-upload behavior are unchanged.

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
