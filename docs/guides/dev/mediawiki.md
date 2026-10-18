<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/MediaWiki?oldid=8308003 (CC BY-SA 4.0); see the page history for contributors. -->

# Work with MediaWiki

You can get a shell as your current user in the main MediaWiki container easily:

```
mw dev mediawiki exec bash
```

You can use the exec command to perform actions inside the MediaWiki container as a one off...

Such as, watching logs

```
mw dev mediawiki exec -- tail -f /var/log/mediawiki/debug.log
```

## Composer download failures

Automatic dependency setup during installation, recipes and `update-code --composer`
retries explicit DNS, connection and download-timeout failures up to three attempts,
waiting 2 seconds and then 5 seconds between attempts. Output stays visible, and
messages show when another attempt is starting. Composer may also retry individual
requests internally.

Dependency conflicts, invalid configuration and script failures still stop the
operation. A failed Composer update stops installation before the database stage.
If network failures persist, check DNS and connectivity inside the MediaWiki
container before rerunning the command. The explicit `mw dev mediawiki composer`
command passes through to Composer without this additional retry policy.

## PHPUnit tests

You can run PHPUnit tests in the MediaWiki container easily:

```
mw dev mediawiki exec -- composer phpunit tests/phpunit/unit/includes/XmlTest.php
```

You may still need to use the old phpunit.php test running for some PHPUnit tests!

```
mw dev mediawiki exec -- php tests/phpunit/phpunit.php tests/phpunit/unit/includes/xml/XmlTest.php
```

If you want to run tests with the debugger, see the XDebug section below.

## XDebug

See [debug PHP with Xdebug](xdebug.md) for IDE setup, triggers, container settings and troubleshooting. Use [Fresh](fresh.md) to run browser tests and record Selenium videos.

## See also

- [User:Santhosh.thottingal/WikiFamily](https://www.mediawiki.org/wiki/User:Santhosh.thottingal/WikiFamily) - Multi wiki setup and how to add new services to the development environment
