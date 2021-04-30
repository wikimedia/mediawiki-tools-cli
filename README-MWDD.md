# MediaWiki CLI - mwdd

mediawiki-docker-dev is being ported into mwcli.

Happy paths are fairly well tested, sad paths may not be, so please keep note of things that you think could be improved.

You can find a built binary at https://github.com/addshore/mwcli/suites/2620593092/artifacts/57429807
This could be considered version `addshore-build-93` (for bug reports etc).

You'll need to extract the binary, make it executable and put it somewhere in your path!

## Usage

Everything should look and feel similar to mwdd or mwdd v1.

**Prerequisites...**

MediaWiki checked out in a directory, with needed skins, extensions and a `composer install` done.

A `LocalSettings.php` file should exist with the following:

```php
<?php
//require_once "$IP/includes/PlatformSettings.php";
require_once '/mwdd/MwddSettings.php';
```

If you want to try using the old mediawiki-docker-dev and the new mwcli mwdd side by side then try the following:

Note: I didn't actually test this yet

```php
<?php
//require_once "$IP/includes/PlatformSettings.php";
if(file_exists('/mwdd/MwddSettings.php')) {
    require_once '/mwdd/MwddSettings.php';
} elseif (file_exists(__DIR__ . '/.docker/LocalSettings.php')) {
    require_once __DIR__ . '/.docker/LocalSettings.php';
} else {
    die('Both mwdd related LocalSettings.php requires failed');
}
```

**Setup...**

You can alter mwdd settings using the cli too.

You **MUST** set **MEDIAWIKI_VOLUMES_CODE** to be your MediaWiki core directory.

```sh
mw mwdd env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
mw mwdd env set PORT 8080
```

And create the basic setup (just MediaWiki):

```sh
mw mwdd create
```

You can check that the needed things are running:

```sh
mw mwdd docker-compose ps
```

**MediaWiki install (basic)...**

You can install a site called "default" using sqlite:

```sh
mw mwdd mediawiki install
```

It should then be accessible at http://default.mediawiki.mwdd.localhost:8080

The sqlite db is stored as part of the `mediawiki` service, so to nuke it you can `destroy`, `create` and `install` this service again.

**MediaWiki install (advanced)...**

You can also specify a site to make and sb type, you may need to start additional services:

```sh
mw mwdd mysql create
mw mwdd mediawiki install mysqlwiki mysql
```

You should see it at http://mysqlwiki.mediawiki.mwdd.localhost:8080

If you wanted to "nuke" your dbs, you can now nuke the single service and recreate the site:

```sh
mw mwdd mysql destroy
mw mwdd mysql create
mw mwdd mediawiki install mysqlwiki mysql
```

**Turning on other individual services...**

Additional services each have their own lifecycle, you can:

```sh
mw mwdd phpmyadmin create
mw mwdd redis create
```

**Container access...**

Services will allow convenient shell access (using your uid by default, or the service user).

Currently the user stuff isn't finished, but you can try shell access with:

```sh
mw mwdd mediawiki exec bash
```

Other convenient commands are being added, such as one for phpunit, though this still needs fixes like (1) paths that are context aware (2) user run options (3) optional alternate wikis, currently default only.

```sh
mwdd mediawiki phpunit /var/www/html/w/tests/phpunit/unit/includes/FormOptionsTest.php
```
