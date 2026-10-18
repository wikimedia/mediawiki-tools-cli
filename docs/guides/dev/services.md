<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/Service_Interaction?oldid=5406037 (CC BY-SA 4.0); see the page history for contributors. -->

# Manage services

Services share common lifecycle commands. For example:

```sh
mw dev memcached create
mw dev memcached stop
mw dev memcached start
mw dev memcached destroy
```

`destroy` removes containers and service data. Use `stop` to keep data for later. Some services also provide tools such as `mw dev mediawiki composer`, `fresh` and `quibble`, or `mw dev redis redis-cli`. Check `mw dev SERVICE --help` for the commands available.

## Inspect configuration

Show the generated Compose configuration or inspect a service's source file:

```sh
mw dev compose config
cat "$(mw dev where)/compose/memcached/compose.yml"
```

## Change environment variables

Set a context variable and recreate the affected service to apply it:

```sh
mw dev env set MEDIAWIKI_XDEBUG_CONFIG "client_host=172.21.112.105"
mw dev mediawiki create
```

Use the address of your debugger. See [MediaWiki debugging](mediawiki.md#xdebug) for more examples.

## Select an image

Inspect the current image, or select a compatible replacement:

```sh
mw dev mediawiki image get
mw dev mediawiki image set REGISTRY/IMAGE:TAG
mw dev mediawiki create
```

Reset the override with `mw dev mediawiki image reset`, then recreate the service. `image dockerfile` commands let you build from your own Dockerfile instead.

## Stop and resume an environment

`mw dev stop` records the running services and stops them. `mw dev start` resumes that saved set. Use `--context NAME` to manage a separate environment.
