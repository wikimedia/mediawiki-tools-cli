#!/usr/bin/env bash

set -e # Fail on errors
set -x # Output commands

# Output some useful docker version infomation
docker --version
docker-compose version

# Output CLI version
./bin/mw version

# Setup things that otherwise need user input
./bin/mw docker env set PORT 8080
./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
# And output their values
./bin/mw docker env list
cat $(./bin/mw docker env where)

# Setup hosts file for used domains
# TODO make the CLI manage this one day https://phabricator.wikimedia.org/T282337
echo "127.0.0.1 default.mediawiki.mwdd.localhost" >> /etc/hosts
echo "127.0.0.1 postgreswiki.mediawiki.mwdd.localhost" >> /etc/hosts
echo "127.0.0.1 mysqlwiki.mediawiki.mwdd.localhost" >> /etc/hosts
echo "127.0.0.1 phpmyadmin.mwdd.localhost" >> /etc/hosts
echo "127.0.0.1 adminer.mwdd.localhost" >> /etc/hosts
cat /etc/hosts

# Create a fresh LocalSettings.php file
rm -f mediawiki/LocalSettings.php
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php