<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/First-Setup?oldid=8232981 (CC BY-SA 4.0); see the page history for contributors. -->

# Create a development environment

[Install the CLI](../installation.md) and Docker Engine 19.03 or newer (API 1.40+) with Docker Compose v2 first. Make sure `docker info` works for your user.

## Create MediaWiki and a database

If you need a MediaWiki checkout, run the code wizard first. It can fetch MediaWiki core and the Vector skin:

```sh
mw dev mediawiki get-code
```

If you already have core and Vector, skip the code wizard and select your checkout when prompted. For an existing `LocalSettings.php`, read the [configuration guide](configuration.md) before reusing it.

Start the MediaWiki containers:

```sh
mw dev mediawiki create
```

Start MySQL and install the wiki:

```sh
mw dev mysql create
mw dev mediawiki install --dbtype=mysql
mw dev status
```

Use the wiki credentials printed by the installer to sign into MediaWiki. They are separate from the gateway login.

## Open the wiki or dashboard

Open the wiki link printed by the installer. You can also reach the default wiki through the gateway at `http://localhost:8080/proxy/default/wiki/Main_Page`. Substitute the port and database name you selected.

Open the development dashboard to see installed wikis and running services:

```sh
mw dev dashboard --open
```

Run `mw dev dashboard` to copy the dashboard URL. See the [gateway guide](gateway.md) for service URLs and signing in over a tunnel.

## Add code and inspect services

```sh
mw dev mediawiki get-code --extension VisualEditor
mw dev mediawiki get-code --skin MinervaNeue
```

Fetching an extension or skin does not enable it. Configure it in your checkout's `LocalSettings.php` and run any required MediaWiki updates.

Use `mw dev SERVICE --help` for service commands. See the existing [service interaction](services.md), [MediaWiki](mediawiki.md) and [database](databases.md) guides for more workflows.

Use [contexts](contexts.md) for separate environments, or [change the HTTP port](ports.md) of an existing environment.

## Wikimedia wiki farm

Use the `wmf-wikifarm` recipe in a dedicated context with a separate MediaWiki checkout.

```sh
wm dev --context wmf-farm env set MEDIAWIKI_VOLUMES_CODE "$PWD/mediawiki-wmf-farm"
wm dev --context wmf-farm recipe --name wmf-wikifarm
```

Open [English Wikipedia](https://en.wikipedia.localhost:8443/wiki/Main_Page),
[Meta-Wiki](https://meta.wikimedia.localhost:8443/wiki/Main_Page), or
[Commons](https://commons.wikimedia.localhost:8443/wiki/Main_Page). Use the demo
account `FarmUser` with password `wikifarm-demo`. Sign in on English Wikipedia,
then visit Meta-Wiki to try the shared login. The private
[Office wiki](https://office.wikimedia.localhost:8443/wiki/Private_farm_page)
uses a separate local account with the same demo credentials.

The HTTPS proxy listens on loopback port `8443` and uses a certificate authority
generated for this context. Export the public certificate if you want to trust
it in a development browser or verify it with `curl`.

```sh
wm dev --context wmf-farm compose cp -- farm-https:/data/caddy/pki/authorities/local/root.crt ./wmf-farm-ca.crt
curl --cacert ./wmf-farm-ca.crt https://en.wikipedia.localhost:8443/wiki/Main_Page
```

The recipe uses Wikimedia registry images for PHP 8.5, Shellbox, Chart, Thumbor,
and ContentTranslation. It builds a local Swift server and reuses a snapshot of
selected Wikimedia wiki settings from [the wiki farm that `wmf-claude` runs in Lima](https://gitlab.wikimedia.org/repos/product-safety-and-integrity/wmf-claude/-/tree/main/templates/wikifarm).
The first application initializes a separate database and search indexes for each wiki.

Swift, Thumbor and the other farm backends are part of this recipe's Compose
configuration. Inspect them through the same context.

```sh
wm dev --context wmf-farm compose ps
wm dev --context wmf-farm compose logs -- swift thumbor cxserver
```

From a CLI source checkout, run the smoke checks against a disposable farm.

```sh
CONTEXT=wmf-farm bash tests/test-docker-recipe-wmf-wikifarm.sh
```
