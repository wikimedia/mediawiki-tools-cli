#!/usr/bin/env bash

# Fail on errors
set -e
# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT
# Output commands
set -x

# Setup the test config
mkdir ~/.mwcli
echo '{"dev_mode":"mwdd"}' > ~/.mwcli/config.json

# Output version
./mw version

# Setup & Create
./mw docker env set PORT 8080
./mw docker env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
./mw docker mediawiki create
./mw docker mysql create
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php

# Validate the basic stuff
./mw docker docker-compose ps
./mw docker env list
cat ~/.mwcli/mwdd/default/.env

# Install & check
./mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Suspend and resume and check the site is still there
./mw docker mysql suspend
./mw docker mysql resume
sleep 2
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Destroy and restart mysql, reinstalling mediawiki
./mw docker mysql destroy
./mw docker mysql create
./mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Destroy it all
./mw docker destroy
docker ps
docker ps | wc -l | grep -q "1"
