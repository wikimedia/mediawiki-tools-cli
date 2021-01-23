# MediaWiki CLI

This project contains a command-line interface for interacting with MediaWiki
development environments.

## Docker

In this initial version there is support for interacting with MediaWiki core's
docker-compose development environment, with subcommands provided under the
`docker` namespace: `mw help docker`.

## Contributing

Clone this repository to your `$GOPATH` (probably `~/go`), so it would be at
`~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli`.

Within the `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/cmd` directory:

- run `go mod download` to download the required modules
- run `go mod vendor` to copy the required modules into a vendor directory for the project

Execute the script from any directory with `go run ~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/cmd/cli/main.go`

### Packages

- `cmd`: Contains the Cobra commands and deals with all CLI user interaction
- `internal/docker`: Logic interacting with the mediawiki-docker dev environment
- `internal/env`: Logic interacting with a `.env` file
- `internal/exec`: Wrapper for the main `exec` package, providing easy verbosity etc
- `internal/mediawiki`: Logic interacting with a MediaWiki directory on disk

### cmd names

No naming structured is enforced in CI but a convention exists that should be followed.

- `root.go` exists as the overall CLI script.
- Top level commands will have their own file in the `cmd` directory, named after the command. Example: `docker.go`.
- Simple sub commands will be defined in those files as vars prefixed with the parent command. For example `dockerStart`.
- Complex sub commands will be split out into their own file. For example `docker_env.go`.
- This is a recursive solution.

### Updating included static files

Static files are included in the binary using <https://github.com/bouk/staticfiles>

You can install the staticfiles command with `go get bou.ke/staticfiles`

In order to update files.go you can run `make internal/mwdd/files/files.go`

### Using a binary

Make a binary by running `make install`

Execute the binary from any directory with `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/bin/mw`

## Support

- Phabricator: [#MediaWiki-Docker](https://phabricator.wikimedia.org/project/view/4585/)
- IRC: `#mediawiki`
