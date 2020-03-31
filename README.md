# MediaWiki CLI

This project contains a command-line interface for interacting with MediaWiki
development environments.

## Docker

In this initial version there is support for interacting with MediaWiki core's
docker-compose development environment, with subcommands provided under the
`docker` namespace: `mw help docker`.

## Contributing

Clone this repository to your `$GOPATH` (probably `~/go`), so it would be at
`~/go/src/cli`.

In the `cli` directory run `go get` to download go package dependencies.

Execute the script with `go run ~/go/src/cli/main.go`

## Support

* Phabricator: [#MediaWiki-Docker](https://phabricator.wikimedia.org/project/view/4585/)
* IRC: `#mediawiki`
