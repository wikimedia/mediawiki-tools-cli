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
