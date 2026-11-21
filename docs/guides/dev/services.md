<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/Service_Interaction?oldid=5406037 (CC BY-SA 4.0); see the page history for contributors. -->

# Manage services

Services share common lifecycle commands. For example:

```sh
wm dev memcached create
wm dev memcached stop
wm dev memcached start
wm dev memcached destroy
```

`destroy` removes containers and service data. Use `stop` to keep data for later. Some services also provide tools such as `wm dev mediawiki composer`, `fresh` and `quibble`, or `wm dev redis redis-cli`. Check `wm dev SERVICE --help` for the commands available.

For optional OpenSearch Dashboards log browsing, see [local logs](logs.md).

## Inspect configuration

Show the generated Compose configuration or inspect a service's source file:

```sh
wm dev compose config
cat "$(wm dev where)/compose/memcached/compose.yml"
```

## Change environment variables

Set a context variable and recreate the affected service to apply it:

```sh
wm dev env set MEDIAWIKI_XDEBUG_CONFIG "client_host=172.21.112.105"
wm dev mediawiki create
```

Use the address of your debugger. See [PHP debugging](xdebug.md) for more examples.

## Select an image

Inspect the current image, or select a compatible replacement:

```sh
wm dev mediawiki image get
wm dev mediawiki image set REGISTRY/IMAGE:TAG
wm dev mediawiki create
```

Reset the override with `wm dev mediawiki image reset`, then recreate the service. See [custom Dockerfiles](dockerfiles.md) to build your own image and apply changes with `rebuild`.

## Stop and resume an environment

`wm dev stop` (alias `suspend`) records the running services before stopping them. `wm dev start` (alias `resume`) resumes that saved set. Services stopped beforehand and completed one-shot setup containers stay stopped. Repeating stop preserves the record; repeating start with nothing left to resume is a no-op.

Service create/start/stop/destroy commands update an existing record. `wm dev restart` targets running services; global destroy removes the record. Direct Docker or Compose commands bypass this tracking. State is separate for each [context](contexts.md).

An older context with no saved stop record initially starts its exited or created services. Its first global stop establishes the selective resume set.

Use `wm dev status` to check the context, gateway and dashboard. For container details or recent web logs, use Compose:

```sh
wm dev start
wm dev status
wm dev compose ps
wm dev compose logs -- --tail=20 mediawiki-web
```

Use `wm dev restart` to restart the currently running services. `wm dev stop` preserves the service set so `wm dev start` can resume it later.
