# MediaWiki CLI - mwdd

A golang mwdd port, incorporated into the mwcli tool.

If you have feedback or requests please write a ticket under the mwcli project https://phabricator.wikimedia.org/tag/mwcli/

## Installing

You can grab built development binaries from https://github.com/addshore/mwcli/releases

If you're on linux and want a quick oneliner:

```sh
sudo wget -O /usr/bin/mw https://github.com/addshore/mwcli/releases/download/v0.1.0-dev-addshore.20210523.1/mw_v0.1.0_linux_amd64 && sudo chmod +x /usr/bin/mw
```

## Migrating from previous mwdd versions

If you want to use a single MediaWiki install with both the new mwcli and the old mediawiki-docker-dev setup, then add this at the top of your LocalSetting.php

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

If you only want to use the new mwcli, then you'll end up with something like this:

```php
<?php
//require_once "$IP/includes/PlatformSettings.php";
require_once '/mwdd/MwddSettings.php';
```

## Usage

Wizards & prompts should guide you through any infomation you need to enter when you run commands.

**Turn on the mediawiki service:**

```sh
mw mwdd mediawiki create
```

**See that the service is running:**

```sh
mw mwdd docker-compose ps
```

**Install a default sqlite site:**

SQLITE is currently used by default...

```sh
mw mwdd mediawiki install
```

It should then be accessible at http://default.mediawiki.mwdd.localhost:8080 (if you are using port 8080)

**Install a mysql site:**

First turn on the mysql service:

```sh
mw mwdd mysql create
```

And then install another site:

```sh
mw mwdd install --dbname mysqlsite --dbtype mysql
```

You can also turn on a postgres service and install postgres sites.

**Turning on other individual services...**

A collection of other services are also available out of the box:

```sh
mw mwdd adminer create
mw mwdd redis create
mw mwdd statsd create
```

**Service commands:**

Most services have a very similar lifecycle.

- `create`, create or update the service
- `suspend`, stop running the service, but keep the data
- `resume`, restart a stopped service
- `destroy`, destroy a service, container and data volumes

**"fancy" commands:**

You can start a shell in the mediawiki container
(This will soon be availbile for other containers [T282394](https://phabricator.wikimedia.org/T282394))

```sh
mw mwdd mediawiki exec bash
```

While in your mediawiki directory, you can easily run phpunit tests

```sh
mw mwdd mediawiki phpunit ./tests/phpunit/unit/includes/FormOptionsTest.php
```

This includes directory aware execution, so if you were already in the tests directory, you could do:

```sh
mw mwdd mediawiki phpunit ./phpunit/unit/includes/FormOptionsTest.php
```

You can also run composer commands
(Running with your composer cache is coming soon [T282336](https://phabricator.wikimedia.org/T282336))

```sh
mw mwdd mediawiki composer info
```

**The guts:**

mwdd now stores data in your home directory in a `.mwcli` directory.

This includes the docker-compose files and .env file.

You can access the .env file via the convenient `mw mwdd env` commands.

If you want to run "raw" docker-compose commands directly on the setup you can use `mw mwdd docker-compose`.
