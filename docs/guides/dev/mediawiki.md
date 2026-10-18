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

The default XDebug mode is `debug`. The image uses `client_host=xdebug://gateway` to find the host gateway; the debugger normally connects on port 9003.

If you want to alter the XDebug config you can do so using an environment variable.

Determine the IP address for the client host by running

```
ip addr
```

Search for the line that says `docker0` and copy the IP address after `inet`. Use that address instead of `1.2.3.4` in the next section:

```
mw dev env set MEDIAWIKI_XDEBUG_MODE "debug,profile"
mw dev env set MEDIAWIKI_XDEBUG_CONFIG "client_host=1.2.3.4" # <-- change this, see above
```

It often makes sense to set the `client_host` to the public IP address of your physical device in your local network (i.e. wifi or ethernet IP).

You can set multiple xDebug config variables by leaving a space between them, for example:

```
mw dev env set MEDIAWIKI_XDEBUG_CONFIG "client_host=172.21.112.105 client_port=9003"
```

If you change these environment variables after the MediaWiki containers are already running, you will need to recreate them to have the settings take effect.

```
mw dev mediawiki create
```

In PHPStorm, make sure you have the option "Ignore external connections through unregistered server configurations" *unchecked*.

### Triggering for requests

See <https://xdebug.org/docs/step_debug#activate_debugger>

Example triggering from CLI inside the MediaWiki container:

```
XDEBUG_SESSION=1 php test.php
```

### Running with tests

You can run exec commands with environment variables, for example, if you want to make use of XDEBUG to step through a specific test in your IDE:

```
mw dev mediawiki exec -- XDEBUG_SESSION=1 PHP_IDE_CONFIG='serverName=<name configured in PHPStorm>' composer phpunit <path to test>
```

## See also

- [User:Santhosh.thottingal/WikiFamily](https://www.mediawiki.org/wiki/User:Santhosh.thottingal/WikiFamily) - Multi wiki setup and how to add new services to the development environment
