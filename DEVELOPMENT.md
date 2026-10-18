# Development Guide

Build, test, and contribute to a Wikimedia CLI.

## Environment Setup

### Requirements

- [Go](https://go.dev/) 1.26+ installed (matching the version in `go.mod`).
- [mise](https://mise.jdx.dev/getting-started.html) for pinned project tools (currently Pandoc for documentation).
- [bingo](https://github.com/bwplotka/bingo) tool (used for managing Go-based build tools).

### Initial Setup

1.  **Clone the repository**:
    Clone it wherever you keep your projects; the checkout does not need to be inside `$GOPATH`.
    ```sh
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

To install the tools pinned in `mise.toml`:

```sh
mise trust
mise install
```

Documentation targets use `mise exec`; activating mise in your shell is optional.

## Building

To build the `mw` binary:

```sh
make build
```

The resulting binary will be located at `./bin/mw`.

### Using the development environment with external hostnames

The development environment normally selects a wiki database from the first label in the request hostname. To serve the default wiki at an additional hostname, configure these comma-separated context variables:

- `MEDIAWIKI_VIRTUAL_HOSTS` controls the wildcard hostnames registered in the local DNS proxy. It defaults to `*.mediawiki.local.wmftest.net`; Traefik routes unmatched hostnames to the default wiki.
- `MEDIAWIKI_DEFAULT_HOSTS` maps exact hostnames to the configured default wiki database (`default` unless changed).
- `MEDIAWIKI_DEFAULT_HTTPS_HOSTS` marks exact hostnames whose public TLS proxy should be used as the wiki's canonical HTTPS URL, preventing redirects to the internal development port.

Add the hostname to `MEDIAWIKI_VIRTUAL_HOSTS` and `MEDIAWIKI_DEFAULT_HOSTS` in the context's `.env` file, then recreate the MediaWiki service. For a hostname served through a TLS proxy, also set `MEDIAWIKI_DEFAULT_HTTPS_HOSTS`.

### Traefik proxy and path gateway

The development environment uses Traefik as its web proxy. It listens on the configured `PORT` (8080 by default), routing the existing local virtual hosts as well as `/proxy/<service>/` paths. MediaWiki keeps its hostname based wiki selection for `*.mediawiki.local.wmftest.net`; the path gateway maps `/proxy/<database>/...` to the matching wiki, and an unmatched hostname uses the default wiki. The dashboard is available at `/proxy/dashboard/`, and web services with Traefik Compose labels are discovered as they start. Set `<SERVICE>_PROXY_PATH` to customize a path route prefix. Authentication is enabled by default; the first Docker service create generates a password. Show it with `mw dev traefik credentials`. Stop the proxy with `mw dev traefik stop` or remove it with `mw dev traefik destroy`.

Proxy routes use a gateway login page and a signed session cookie that expires after 12 hours. The first Docker service `create` generates a random password for the `wm` user and saves it with owner-only permissions; show it with `mw dev traefik credentials`. The credentials command also prints single-use login links for localhost and any active Cloudflare Quick Tunnel, valid for five minutes; use `--url` to select a different gateway URL. Existing credentials are preserved on upgrade. Sign out through the dashboard or `/_wm/logout`. Application logins and Bearer tokens work independently of the gateway session. Scripts can still use HTTP Basic credentials, for example `curl -u wm:PASSWORD http://localhost:8080/proxy/dashboard/api/status`. To disable proxy authentication for a context, set `TRAEFIK_AUTH_MIDDLEWARE=forwarded-https@file` in its `.env` and recreate the web services. The login service and GET-only Docker API proxy are internal and publish no ports. Gateway URLs use `/_wm/`; old `/_mwcli/` links and signed session cookies remain supported during upgrades. Run `mw dev compose up -- -d --force-recreate gateway-auth traefik` after updating the CLI to load the new gateway code.

To expose the gateway from a development machine that is not directly reachable, start an optional tunnel. Cloudflare remains the default:

```sh
mw dev tunnel start
mw dev tunnel start --provider holesail
```

Select `cloudflare` or `holesail` with `--provider`, or set the context default with `mw dev env set TUNNEL_PROVIDER holesail`. A successful `start` remembers the provider and stops any running tunnel using the other provider. If startup fails, the previous tunnel remains running. `mw dev tunnel status`, `url`, and `logs --follow` use the saved provider; `--provider` overrides it for a command. `mw dev tunnel stop` stops both providers while leaving the gateway and wiki running. `stop --provider holesail` stops only Holesail. Gateway authentication remains enabled by default with either provider.

Cloudflare prints a temporary public `trycloudflare.com` HTTPS URL that works in an ordinary browser. Quick Tunnels do not guarantee uptime, have a 200 in-flight request limit, and do not support Server-Sent Events (SSE).

Holesail prints a private `hs://s000...` connection string. Treat it as a secret. The optional container is built on first use with Holesail 2.4.4 and locked npm dependencies; nothing needs installing on the server host. On the connecting computer, install Holesail and run the client command printed by `start`, such as:

```sh
npm install --global holesail@2.4.4
holesail 'hs://s000...' --host 127.0.0.1 --port 8082
```

Then open `http://127.0.0.1:8082/proxy/dashboard/` or `http://127.0.0.1:8082/proxy/default/wiki/Main_Page`, using the gateway login from `mw dev traefik credentials`. The local port can be changed. Holesail does not provide a public browser URL: every connecting computer needs a client. Connections require working peer discovery and peer connectivity on the network. Server restarts generate a new connection string; use `mw dev tunnel url` to retrieve it. HTTP links use the local client origin; Cloudflare and other HTTPS proxies retain their forwarded HTTPS origin.

For an end-to-end Holesail check on a machine with Docker, curl, Python 3, Linux host networking, and public DHT access, run `./tests/test-docker-tunnel-holesail.sh`. It uses a separate client container with Linux host networking on the same VM and checks gateway authentication, the dashboard, wiki pages and the canonical URL. Set `MW_BIN`, `TUNNEL_CONTEXT`, or `CLIENT_PORT` to override its defaults. It stops its test tunnel and client, and restores the previous provider setting. The GitLab request plan selects `docker-tunnel-holesail` on `linux-amd64`; the shared GitHub executor runs it with a disposable context and prepared default wiki. The real public DHT path is checked end to end, alongside the offline lifecycle tests.

Traefik binds the existing `PORT` on all interfaces, so local virtual hosts and remote `/proxy/...` routes use the same listener. The root path on an unmatched host serves the default wiki behind the same authentication. Some tools require app-specific base-path settings for all links and assets to work under a prefix; prefix routing alone does not guarantee that every tool's UI is fully compatible.

The dashboard and gateway sign-in/sign-out pages use Wikimedia Codex 2.7.0 CSS-only components. Component styles, design tokens (including automatic dark mode), and licenses are bundled under `mount/dev/compose/base/mount/ui/` and served locally. No frontend build or external CDN is needed at runtime. `app.css` handles application layout using Codex tokens. To refresh the pinned vendor files, run `npm ci --prefix tools/ui` (Node.js 18+), then `python3 tools/ui/update-codex.py` followed by `make generate`; upgrade instructions are in the assets directory's README. When running a Go server directly, set `MWCLI_UI_ASSETS` to that directory. After updating an existing context, recreate the dashboard and gateway-auth containers to pick up the shared read-only `/ui` mount.

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
Its `--jump` flag and script-upload behavior are unchanged. Configure OpenStack credentials with `mw cloudvps auth add`; it links to Horizon and prompts for missing values, hiding the secret. Project prompts and `--project` flags support Tab completion using cached public inventory, with a short lookup timeout and configured projects as fallbacks.

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
- `make linti`: Check command help and metadata.
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
    - `internal/devenv`: Package for the docker-compose powered development environment.
    - `internal/toolhub`: Client for interacting with toolhub.wikimedia.org.
    - `internal/updater`: Code for updating the CLI.
    - `internal/util`: DEPRECATED: Independent packages (slowly being moved to `pkg/`).
- `pkg/`: Reusable packages independent of the CLI.
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
They are embedded into the binary and extracted into your development context under `recipes/`.

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

To review the diff:

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

Markdown guides in `docs/` and generated command help are published to MediaWiki.org from release tags. See [documentation publishing](docs/README.md) for page mappings, imports and credentials.

```sh
make docs        # Build Markdown and wikitext locally
make docs-check  # Show wiki edits since the last automatic publication
make docs-audit  # Find wiki pages outside the current build
make docs-pull   # Download wiki content and Markdown proposals for review
```

Wiki edits block automatic publication until reviewed. Documentation artifacts include the current content and diffs; edits also use revision IDs to protect changes made after the check.

### Naming and releases

The display brand is `a Wikimedia CLI`; `mw` remains the default command name. `make BINARY_NAME=wm build` builds `bin/wm`, and `make BINARY_NAME=wm release` selects the release filename prefix. Help and shell completion use the invoked executable name; generated documentation uses `cli.DefaultCommandName`. Configuration paths, `MWCLI_` environment variables, telemetry streams, recipe schemas and managed-file markers, Docker resource names, package-registry identity, and the Go module path retain their existing identities during the transition. Cloud VPS project names, CI runner tags, and Phabricator tags refer to existing external resources.

Each platform has one uploaded executable and checksum. When publishing `wm_…` assets, release metadata adds legacy `mw_…` links through the canonical release download URL, preserving old updaters without uploading duplicate binaries. New updaters accept either prefix and either `bin/mw` or `bin/wm` in CI archives (`.exe` on Windows); updates preserve the installed executable's path and name.

Use `mw dev` for the development environment (`docker` remains a command alias). Its Go packages are `commands/dev` and `internal/devenv`. Existing context directories, Compose project names and installed wiki credentials keep their legacy identities; Generated `LocalSettings.php` files retain `/mwdd/MwddSettings.php` as a stable entry point: current releases forward it to `DevSettings.php`, while older releases restore it during file sync. Container startup and helper paths retain their existing mounts so previously created containers can still restart.

## Toolforge commands

`mw tools` (also `mw toolforge`) embeds the upstream Toolforge CLI command tree.
Select an account with `--tool`; jobs, builds, environment variables, components,
and Kubernetes commands use upstream's authentication and SSH tunneling.

`mw tools shell --tool mytool` opens an interactive tool login shell. Its `exec`
alias preserves the existing command spelling. Put `--` before remote commands
with flags; use `sh -c` explicitly for remote shell pipelines or redirection.
Arguments are otherwise passed literally, and SSH failures retain their exit code.

`mw tools cp --tool mytool source... destination` uploads one or more files using
rsync. Both commands support toolsbeta and lima-kilo through `--beta` and `--lima`.
File transfer requires local `rsync` and `ssh`; other commands do not require a
separately installed Toolforge executable.
