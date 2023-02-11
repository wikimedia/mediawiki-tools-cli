# Docker development environment

A docker based MediaWiki development environment.

You can start a basic MediaWiki MySQL development environment with the following commands:

```sh
mw docker mediawiki create
mw docker mysql create
mw docker mediawiki install --dbtype=mysql
```

You can:
 - Suplement this environment with additional services over time (See "Service commands" such as `redis`)
 - Recreate the entire environment in seconds (See `destroy`)
 - Run sites (See `mediawiki install`) and multiple environments simultaniously (See `--context`)
 - Run all common developer tools inside docker with ease (Eg. `mediawiki composer` / `fresh` / `quibble`)