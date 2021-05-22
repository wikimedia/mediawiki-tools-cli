#!/usr/bin/env bash

# Fail on errors
set -e
# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT
# Output commands
set -x

# Setup & Create
./mw mwdd env set PORT 8080
./mw mwdd env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
./mw mwdd create

# Validate the basic stuff
./mw mwdd docker-compose ps
./mw mwdd env list
cat ~/.mwcli/mwdd/default/.env
curl -s -L -N http://default.mediawiki.mwdd.localhost:8080 | grep -q "The MediaWiki logo"

# Add the needed LocalSettings
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php

# Install sqlite & check
./mw mwdd mediawiki install
curl -s -L -N http://default.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Turn on mysql, install & check
./mw mwdd mysql create
./mw mwdd mediawiki install --dbname mysqlwiki --dbtype mysql
curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Turn on postgres, install & check
./mw mwdd postgres create
./mw mwdd mediawiki install --dbname postgreswiki --dbtype postgres
curl -s -L -N http://postgreswiki.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Turn on the db management services
./mw mwdd phpmyadmin create
./mw mwdd adminer create
sleep 2
curl -s -L -N http://phpmyadmin.mwdd.localhost:8080 | grep -q "Open new phpMyAdmin window"
curl -s -L -N http://adminer.mwdd.localhost:8080 | grep -q "Login - Adminer"

# Make sure the expected number of services appear
docker ps
docker ps | wc -l | grep -q "9"

# Destroy it all
./mw mwdd destroy
# And make sure only 1 exists after
docker ps
docker ps | wc -l | grep -q "1"
