# MediaWiki CLI

This project contains a command-line interface for interacting with MediaWiki
development environments.

Take a look at the user facing docs https://www.mediawiki.org/wiki/Cli

## Docker

There are currently 2 subcommands:

- `docker` allows interacting with MediaWiki core's docker-compose development environment. (See `mw help docker`)
- `mwdd` allows interacting with a new version of the MediaWiki-docker-dev development environment. (See `mw help mwdd`)

## Contributing

Clone this repository to your `$GOPATH` (probably `~/go`), so it would be at
`~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli`.

Within the `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/cmd` directory:

- run `make` to download dependencies and build an initial binary

Execute the tool without building from any directory by running the `./dev.sh` script.

### Packages & Directories

- `cmd`: Contains the Cobra commands and deals with all CLI user interaction.
- `internal/cmd`: General Cobra command abstractions that may be useful in multiple places.
- `internal/docker`: Logic interacting with the mediawiki-docker dev environment.
- `internal/env`: Logic interacting with a `.env` file.
- `internal/exec`: Wrapper for the main `exec` package, providing easy verbosity etc.
- `internal/mediawiki`: Logic interacting with a MediaWiki installation directory on disk.
- `internal/mwdd`: Logic for the MediaWiki-docker-dev development environment.
- `static`: Files that end up being built into the binary.

### cmd names

No naming structured is enforced in CI but a convention exists that should be followed.

- `root.go` exists as the overall CLI script.
- Top level commands will have their own file in the `cmd` directory, named after the command. Example: `docker.go`.
- Simple sub commands will be defined in those files as vars prefixed with the parent command. For example `dockerStart`.
- Complex sub commands will be split out into their own file. For example `docker_env.go`.
- This is a recursive solution.

### Using a binary

Make a binary by running `make`

Execute the binary from any directory with `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/bin/cli`

## Support

- Documentation: [Cli page on mediawiki.org](https://www.mediawiki.org/wiki/Cli)
- Phabricator: [#MediaWiki-Docker](https://phabricator.wikimedia.org/project/view/4585/)
- IRC: `#mediawiki`
