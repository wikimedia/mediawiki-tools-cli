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

Use the address of your debugger. See [PHP debugging](xdebug.md) for more examples.

## Select an image

Inspect the current image, or select a compatible replacement:

```sh
mw dev mediawiki image get
mw dev mediawiki image set REGISTRY/IMAGE:TAG
mw dev mediawiki create
```

Reset the override with `mw dev mediawiki image reset`, then recreate the service. See [custom Dockerfiles](dockerfiles.md) to build your own image and apply changes with `rebuild`.

## Stop and resume an environment

`mw dev stop` (alias `suspend`) records the running services before stopping them. `mw dev start` (alias `resume`) resumes that saved set. Services stopped beforehand and completed one-shot setup containers stay stopped. Repeating stop preserves the record; repeating start with nothing left to resume is a no-op.

Service create/start/stop/destroy commands update an existing record. `mw dev restart` targets running services; global destroy removes the record. Direct Docker or Compose commands bypass this tracking. State is separate for each [context](contexts.md).

An older context with no saved stop record initially starts its exited or created services. Its first global stop establishes the selective resume set.

For a quick environment check, inspect the Compose services and recent web logs after starting:

```sh
mw dev start
mw dev compose ps
mw dev compose logs -- --tail=20 mediawiki-web
```

Use `mw dev restart` to restart the currently running services. `mw dev stop` preserves the service set so `mw dev start` can resume it later.
