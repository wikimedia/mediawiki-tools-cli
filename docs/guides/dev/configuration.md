<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/LocalSettings.php?oldid=5181877 (CC BY-SA 4.0); see the page history for contributors. -->

# Configure MediaWiki

The development environment supplies database connections, service endpoints, logging and other settings through a shared settings file. Your checkout's `LocalSettings.php` loads that file before your own overrides:

```php
<?php
require_once '/mwdd/MwddSettings.php';
```

The [settings source](https://gitlab.wikimedia.org/repos/releng/cli/-/blob/main/mount/dev/compose/mediawiki/mount/mediawiki/DevSettings.php) shows the defaults. The `/mwdd/MwddSettings.php` entry point is retained so existing containers and downgrades keep working. In current releases it forwards to `DevSettings.php`; older releases restore their own settings at that same path.

## Add settings and extensions

Put your settings after the shared file so they can override its defaults:

```php
<?php
require_once '/mwdd/MwddSettings.php';

wfLoadSkin( 'Vector' );
wfLoadExtension( 'BoilerPlate' );

$wgEnableParserCache = false;
```

Download extensions and skins with `mw dev mediawiki get-code` before enabling them. Run any required schema updates afterward:

```sh
mw dev mediawiki mwscript update
```

## Reuse an existing checkout

Keep application settings and extension configuration, but remove conflicting database and service settings that the shared file supplies. See [setup](setup.md) and [database services](databases.md).

Older generated files may include a commented `PlatformSettings.php` line. The shared settings file loads the platform settings itself.
