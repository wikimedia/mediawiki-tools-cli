# Change the HTTP port

`PORT` controls Traefik's published host port and the HTTP listener inside its container. It defaults to 8080, or another free port chosen during setup. Apache and other services keep their own internal ports.

## Find the saved port

Each development context stores `PORT` in its own `.env` file. Show the saved
value and the location of that file with:

```sh
mw dev env get PORT
mw dev env where
```

Destroying containers preserves this file. Recreating them reuses the saved port,
so the initial port prompt does not appear again.

## Set and apply a port

For an existing environment, choose an unused port and inspect the configuration:

```sh
mw dev env set PORT 8088
mw dev compose config
```

Recreate Traefik and the dashboard to apply their new port settings:

```sh
mw dev compose up -- -d --force-recreate traefik dashboard
mw dev traefik credentials
```

Open the new login link or `http://default.mediawiki.local.wmftest.net:8088/wiki/Main_Page`. The gateway dashboard is at `http://localhost:8088/proxy/dashboard/`. Changing `.env` alone does not change a running container's port bindings.

Use the same `--context NAME` for each command when targeting another [context](contexts.md). To choose a port before initial setup, set it before `mw dev mediawiki create`.

## Update tunnels and custom URLs

Restart any running [tunnel](gateway.md) so it targets the new gateway port:

```sh
mw dev tunnel stop
mw dev tunnel start
```

A temporary public tunnel can receive a different URL after restarting. Update manually configured URLs or application settings that contain the previous port. Fresh browser tests use the internal MediaWiki service address and do not need a new `MW_SERVER` merely because the host port changed.

`MYSQL_PORT_3306` exposes the database separately; it is not the HTTP port. `MW_DOCKER_PORT` is not a setting used by this CLI.
