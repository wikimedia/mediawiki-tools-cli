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
./mw docker env set PORT 8080
./mw docker env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
./mw docker mediawiki create

# Validate the basic stuff
./mw docker docker-compose ps
./mw docker env list
cat ~/.mwcli/mwdd/default/.env
curl -s -L -N http://default.mediawiki.mwdd.localhost:8080 | grep -q "The MediaWiki logo"

# Add the needed LocalSettings
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php

# Turn on all of the services
./mw docker mysql-replica create
./mw docker postgres create
./mw docker phpmyadmin create
./mw docker adminer create

# Install everything
./mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
./mw docker mediawiki install --dbname postgreswiki --dbtype postgres
./mw docker mediawiki install

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
./mw docker destroy
# And make sure only 1 exists after
docker ps
docker ps | wc -l | grep -q "1"
