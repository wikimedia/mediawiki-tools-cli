# MediaWiki CLI

This project contains a command-line interface for interacting with MediaWiki
development environments.

Take a look at the user facing docs https://www.mediawiki.org/wiki/Cli

## Contributing

Clone this repository to your `$GOPATH` (probably `~/go`), so it would be at
`~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli`.

Within the `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli` directory:

Run `make` to build a binary to `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/bin/mw`.

We recommend that you create a development alias for this binary, and run `make` after you make changes to the codebase.

```sh
alias mwdev='~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/bin/mw'
```

### Makefile commands

Many other Makefile commands exist that you might find useful:

- `make build`: Just builds a new binary
- `make release`: Builds multiple release binaries to `_release`
- `make test`: Run unit tests
- `make lint`: Run basic linting
- `make vet`: Run `go vet`
- `make staticcheck`: Run https://staticcheck.io/

### Packages & Directories

- `cmd`: Contains the Cobra commands and deals with all CLI user interaction.
- `internal/cmd`: General Cobra command abstractions that may be useful in multiple places.
- `internal/config`: Interaction with the CLI configuration
- `internal/docker`: Logic interacting with the mediawiki-docker dev environment.
- `internal/env`: Logic interacting with a `.env` file.
- `internal/exec`: Wrapper for the main `exec` package, providing easy verbosity etc.
- `internal/gitlab`: Basic interaction with the Wikimedia Gitlab instance
- `internal/mediawiki`: Logic interacting with a MediaWiki installation directory on disk.
- `internal/mwdd`: Logic for the MediaWiki-docker-dev development environment.
- `internal/updater`: CLI tool updating.
- `static`: Files that end up being built into the binary.
- `tests`: Integration tests that are run as part of CI.

### cmd names

No naming structured is enforced in CI but a convention exists that should be followed.

- `root.go` exists as the overall CLI script.
- Major commands will have their own file in the `cmd` directory, named after the command. Example: `mwdd.go`.
- The lowest level commands can be included in their parent files. Example `mwdd_mediawiki.go` contains subcommands such as `mwddMediawikiCmd`.
- Complex sub commands can be split out into their own file.
- This is a recursive solution.

## Support

- Documentation: [Cli page on mediawiki.org](https://www.mediawiki.org/wiki/Cli)
- Phabricator: [#mwcli](https://phabricator.wikimedia.org/project/view/5331/)
- IRC: `#mediawiki`
