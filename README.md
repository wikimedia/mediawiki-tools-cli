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

If you want to contribute tp this repository, please [ask an existing maintainer](https://gitlab.wikimedia.org/repos/releng/cli/-/project_members) to be added as a developer.

Once this has happened you will be able to make branches in this repository and CI will run for you.

If you create merge requests from forks, CI will not run.

You can request access to the project by [filing a ticket against the #mwcli project on phabricator](https://phabricator.wikimedia.org/maniphest/task/edit/form/1/?tags=mwcli&title=Request%20access%20to%20mwcli%20gitlab%20project%20for%20%3CUSER%3E).

### Repo / Code setup

You need go 1.18+ installed.

This repository uses the `bingo` tool.
You can install it with:

```sh
go install github.com/bwplotka/bingo@v0.7.0
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
- `make lint`: Run basic go linting
- `make linti`: Run custom mwcli command linting (lint internal)
- `make fix`: Run basic lint fixes
- `make vet`: Run `go vet`
- `make staticcheck`: Run https://staticcheck.io/

### Packages & Directories

#### CLI wide general packages

- `cmd`: Creation and execution of the top level Cobra command.
- `internal/cli`: High level things used across the CLI.
- `internal/cmd`: Packages for commands that make up part of the CLI, binding to cobra.
- `internal/cmdgloss`: Glossy output for users
- `internal/codesearch`: Client for interacting with https://codesearch.wikimedia.org
- `internal/config`: CLI wide configuration.
- `internal/eventlogging`: Client to submit events to Wikimedia Event Logging
- `internal/exec`: Wrapper around `exec` for easily running commands and capturing output. TODO clean this up
- `internal/gitlab`: Client for interacting with https://gitlab.wikimedia.org
- `internal/mediawiki`: Interact with a MediaWiki installation directory on disk
- `internal/mwdd`: Package for the docker-compose powered development environment
- `internal/toolhub`: Client for interacting with https://toolhub.wikimedia.org
- `internal/updater`: Code for updating the CLI.
- `internal/util`: DEPRECATED: Independent packages that do not bind the CLI in any way. (Slowly bring moved to `./pkg`)
- `pkg`: Independently useful packages that do not bind to mwcli code or concepts.
- `tests`: Integration tests that are run as part of CI.
- `tools`: Various tools to make working with this repository easier.

## CI & Integration tests

This repository has continuous integration setup on Gitlab.
You can read more in the [CI README](./CI.md).

You can also choose to run the integration tests locally.

```sh
./tests/test-general.sh
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
    - Notes should be under a fresh new header of the format `## v0.2.1` so that the release process can extract the notes correctly. These are displayed to users as they update.
2) Tag the commit for release
    - The format should be `vx.x.x`
    - The `v` prefix is needed for the release CI to run for the tag
3) [Watch the pipeline run](https://gitlab.wikimedia.org/repos/releng/cli/-/pipelines) that is building, uploading and publishing the release.
4) Check that the release appear [on the releases page](https://gitlab.wikimedia.org/repos/releng/cli/-/releases)
5) You should now be able to run `mw update` to grab the latest release.
6) Update the version in the [installation docs code snippets](https://www.mediawiki.org/wiki/Cli/guide/Installation)

## Docs

Docs for mediawiki.org are automatically generated by CI when a release is made.
They can also be manually generated from this repository and pushed to mediawiki.org

Note: You will need `pandoc` installed. https://pandoc.org/

```sh
make docs
```

If you also want to publish them, you'll need something like this:

```sh
make user="someUser" password="somePassword" docs-publish
```

You can use a bot password for this https://mediawiki.org/wiki/Special:BotPasswords \
The bot will need at least the `Basic rights` and `High-volume editing` permissions.
