<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/First-Setup?oldid=8232981 (CC BY-SA 4.0); see the page history for contributors. -->

# Create a development environment

[Install the CLI](../installation.md) and Docker with Docker Compose first. Make sure `docker info` works for your user.

## Create MediaWiki and a database

Start the setup wizard:

```sh
mw dev mediawiki create
```

Choose a local port, a MediaWiki checkout and cloning options. You can use an existing checkout or let the wizard fetch one. For an existing `LocalSettings.php`, read the [configuration guide](configuration.md) before reusing it.

Start MySQL and install the wiki:

```sh
mw dev mysql create
mw dev mediawiki install --dbtype=mysql
mw dev compose ps
```

Use the wiki credentials printed by the installer to sign into MediaWiki. They are separate from the gateway login.

## Open the wiki

The default port is 8080. Open `http://default.mediawiki.local.wmftest.net:8080/wiki/Main_Page`, or use `http://localhost:8080/proxy/default/wiki/Main_Page` through the gateway. Substitute the port and database name you selected.

Generate a gateway login link:

```sh
mw dev traefik credentials
```

Open its `Login` link, then visit `http://localhost:8080/proxy/dashboard/` to find the running services. The [gateway guide](gateway.md) explains login sessions, service URLs and tunnels.

## Add code and inspect services

```sh
mw dev mediawiki get-code --extension VisualEditor
mw dev mediawiki get-code --skin MinervaNeue
```

Fetching an extension or skin does not enable it. Configure it in your checkout's `LocalSettings.php` and run any required MediaWiki updates.

Use `mw dev SERVICE --help` for service commands. See the existing [service interaction](services.md), [MediaWiki](mediawiki.md) and [database](databases.md) guides for more workflows.
