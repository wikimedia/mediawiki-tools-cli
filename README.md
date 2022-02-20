# MediaWiki CLI

This project contains a command-line interface for MediaWiki and Wikimedia developers.

It includes a MediaWiki development environment modeled after [mediawiki-docker-dev](https://www.mediawiki.org/wiki/MediaWiki-Docker-Dev).

Take a look at the user facing docs https://www.mediawiki.org/wiki/Cli

## Support

- Code Repository: [releng/cli on gitlab.wikimedia.org](https://gitlab.wikimedia.org/repos/releng/cli)
- Documentation: [Cli page on mediawiki.org](https://www.mediawiki.org/wiki/Cli)
- Phabricator: [#mwcli on phabricator.wikimedia.org](https://phabricator.wikimedia.org/project/view/5331/)
- IRC: `#mediawiki` on [Libera.​Chat](https://libera.chat/)

## Contributing

### Repo / Code setup

You need go 1.16+ installed.

This repository uses the `bingo` tool.
You can install it with:

```sh
go install github.com/bwplotka/bingo@latest
```

Clone this repository to your `$GOPATH` (probably `~/go`), so it would be at
`~/go/src/gitlab.wikimedia.org/repos/releng/cli`.

Within the `~/go/src/gitlab.wikimedia.org/repos/releng/cli` directory:

You can install all tools used by this repository using bingo.

```sh
bingo get
```

You can then build a binary

```sh
make build
```

Which you will find at `~/go/src/gitlab.wikimedia.org/repos/releng/cli/bin/mw`.

We recommend that you create a development alias for this binary, and run `make` after you make changes to the codebase.

```sh
alias mwdev='~/go/src/gitlab.wikimedia.org/repos/releng/cli/bin/mw'
```

### Makefile commands

Many other Makefile commands exist that you might find useful:

- `make build`: Builds a new binary
- `make release`: Builds multiple release binaries to `_release`
- `make test`: Run unit tests
- `make lint`: Run basic linting
- `make fix`: Run basic lint fixes
- `make vet`: Run `go vet`
- `make staticcheck`: Run https://staticcheck.io/

### Packages & Directories

- `cmd`: Creationand execution of the top level mw cobra CLI command.
- `internal/cli`: High level things used across the CLI.
- `internal/util`: Independant packages that do not bind the the CLI.
- `internal/cmd`: Packages for commands that make up part of the CLI, binding to cobra.
- `internal/<command name>`: Packages for commands that should not bind to cobra.
- `tests`: Integration tests that are run as part of CI.
- `tools`: Various tools to make working with this repository easier.

## CI & Integration tests

This repository has continious integration setup on Gitlab.
You can read more in the [CI README](./CI.md).

You can also choose to run the integration tests locally.

```sh
./tests/test-general-commands.sh
```

Or for the dev environment

```sh
./tests/test-docker-general.sh
./tests/test-docker-mw-all-dbs.sh
./tests/test-docker-mw-mysql-cycle.sh
```

These tests should clean up after themselves.
If you run into issues you might find `./tests/destroy.sh` useful.

## Releasing

Releases are automatically built and published by Gitlab CI after pushing a tag.

Tags should follow [semver](https://semver.org/) and release notes should be written prior to tagging.

### Process

1) Add release notes for the release into CHANGELOG.md
    - You can use a compare link such as [this](https://gitlab.wikimedia.org/repos/releng/cli/-/compare/v0.10.0...main?from_project_id=16) to see what has changed and what needs release notes.
    - Notes should be under a fresh new header of the format `## v0.2.1` so that the release process can extract the notes correctly.
2) Tag & push the commit
3) [Watch the pipeline run](https://gitlab.wikimedia.org/repos/releng/cli/-/pipelines) that is building, uploading and publishing the release.
4) Check that the release appear [on the releases page](https://gitlab.wikimedia.org/repos/releng/cli/-/releases)
5) Publish up to date ref docs (see below)
6) You should now be able to run `mw update` to grab the latest release.

## Docs

Docs for mediawiki.org can be automatically generated from this repository.

Note: You will need `pandoc` installed. https://pandoc.org/

```sh
make docs
```

If you also want to publish them, you'll need something like this:

```sh
make user="someUser" password="somePassword" docs-publish
```

You can use a bot password for this https://mediawiki.org/wiki/Special:BotPasswords

In the future this would be done by CI!