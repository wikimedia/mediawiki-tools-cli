#!/usr/bin/env bash

# Fail on errors
set -e
# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT
# Output commands
set -x

# Output version
./mw version

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

# Turn on all of the services
./mw mwdd mysql-replica create
./mw mwdd postgres create
./mw mwdd phpmyadmin create
./mw mwdd adminer create

# Install everything
./mw mwdd mediawiki install --dbname mysqlwiki --dbtype mysql
./mw mwdd mediawiki install --dbname postgreswiki --dbtype postgres
./mw mwdd mediawiki install

# Check the DB tools
CURL=$(curl -s -L -N http://phpmyadmin.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "Open new phpMyAdmin window"
CURL=$(curl -s -L -N http://adminer.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "Login - Adminer"

# And check the installed sites
CURL=$(curl -s -L -N http://default.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"
CURL=$(curl -s -L -N http://postgreswiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Make sure the expected number of services appear
docker ps
docker ps | wc -l | grep -q "10"

# Destroy it all
./mw mwdd destroy
# And make sure only 1 exists after
docker ps
docker ps | wc -l | grep -q "1"
