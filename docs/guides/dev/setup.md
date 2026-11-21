<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/First-Setup?oldid=8232981 (CC BY-SA 4.0); see the page history for contributors. -->

# Create a development environment

[Install the CLI](../installation.md) and Docker Engine 19.03 or newer (API 1.40+) with Docker Compose v2 first. Make sure `docker info` works for your user.

## Create MediaWiki and a database

If you need a MediaWiki checkout, run the code wizard first. It can fetch MediaWiki core and the Vector skin:

```sh
wm dev mediawiki get-code
```

If you already have core and Vector, skip the code wizard and select your checkout when prompted. For an existing `LocalSettings.php`, read the [configuration guide](configuration.md) before reusing it.

Start the MediaWiki containers:

```sh
wm dev mediawiki create
```

Start MySQL and install the wiki:

```sh
wm dev mysql create
wm dev mediawiki install --dbtype=mysql
wm dev status
```

Use the wiki credentials printed by the installer to sign into MediaWiki. They are separate from the gateway login.

## Open the wiki or dashboard

Open the wiki link printed by the installer. You can also reach the default wiki through the gateway at `http://localhost:8080/proxy/default/wiki/Main_Page`. Substitute the port and database name you selected.

Open the development dashboard to see installed wikis and running services:

```sh
wm dev dashboard --open
```

Run `wm dev dashboard` to copy the dashboard URL. See the [gateway guide](gateway.md) for service URLs and signing in over a tunnel.

## Add code and inspect services

```sh
wm dev mediawiki get-code --extension VisualEditor
wm dev mediawiki get-code --skin MinervaNeue
```

Fetching an extension or skin does not enable it. Configure it in your checkout's `LocalSettings.php` and run any required MediaWiki updates.

Use `wm dev SERVICE --help` for service commands. See the existing [service interaction](services.md), [MediaWiki](mediawiki.md) and [database](databases.md) guides for more workflows.

Use [contexts](contexts.md) for separate environments, or [change the HTTP port](ports.md) of an existing environment.
