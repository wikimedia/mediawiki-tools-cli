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

### Using a binary

Make a binary by running `make install`

Execute the binary from any directory with `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/bin/mw`

## Support

- Phabricator: [#MediaWiki-Docker](https://phabricator.wikimedia.org/project/view/4585/)
- IRC: `#mediawiki`
