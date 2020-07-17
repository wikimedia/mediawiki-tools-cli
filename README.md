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

In the `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/cmd` directory run `go get` to download go package dependencies.

Execute the script with `go run ~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/cmd/cli/main.go`

In order to run the script within the MediaWiki directory you'll need to install it `make install`

And then run the build binary `~/go/src/gerrit.wikimedia.org/r/mediawiki/tools/cli/bin/mw`

## Support

* Phabricator: [#MediaWiki-Docker](https://phabricator.wikimedia.org/project/view/4585/)
* IRC: `#mediawiki`
