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

For optional OpenSearch Dashboards log browsing, see [local logs](logs.md).

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

The CLI records which existing Compose services you intend to keep running and
which you explicitly stopped. This state is separate for each [context](contexts.md).

- Service `create`, `rebuild` and `start` record running intent after success.
- Service `stop` records stopped intent; `destroy` removes the service record.
- `mw dev stop` (alias `suspend`) stops running containers while preserving intent.
- `mw dev start` (alias `resume`) starts existing exited/created containers marked
  running. It does not recreate missing containers or start stopped services.
- `mw dev restart` targets currently running long-lived services only.
- `mw dev update` pulls images for existing long-lived services, then updates only
  running containers, without starting their dependencies or stopped services.
- Global `destroy` removes service intent. Recipe service actions use the same state.

Long-running dependencies started by a service create/start are recorded too.
Completed setup jobs such as MySQL replication configuration are excluded from
resume/restart by their `org.wikimedia.cli.lifecycle: oneshot` Compose label;
no decision is based on a container's exit code. Explicit create/start/rebuild may run
those jobs again as part of provisioning.

The versioned `state-suspended-services.json` file stores desired running and
stopped sets, with one-shot roles. It does not contain `.env` values, credentials,
wiki installation details or live Docker status. OS-backed locks serialize CLI
lifecycle operations and state writes within a context. Direct Docker/Compose
commands bypass this tracking and do not change saved intent.

### Existing environments

An existing suspend snapshot is migrated with its resume set preserved. When no
snapshot exists, the CLI adopts currently running containers as intended running
and existing stopped containers as intended stopped. Exited containers cannot
reliably reveal historical user intent, so they are not all started automatically.
A warning explains this the first time a stopped legacy environment is adopted.
Use `mw dev SERVICE start` for each service you want to run thereafter.

A failed service operation leaves its previous intent intact, even if Docker
partially changed containers; inspect Docker's reported error before retrying.
Global stop records its pending operation before stopping containers, so retrying
an interrupted or failed stop retains the desired resume set. Repeated start/stop
operations do not erase intent when all containers are already stopped.

For a quick environment check, inspect the Compose services and recent web logs after starting:

```sh
mw dev start
mw dev compose ps
mw dev compose logs -- --tail=20 mediawiki-web
```
