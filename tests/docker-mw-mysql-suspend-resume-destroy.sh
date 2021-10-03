#!/usr/bin/env bash

set -e # Fail on errors
set -x # Output commands

# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT

# Set some corret values so we don't get asked
./bin/mw docker env set PORT 8080
./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki

# Setup the default hosts in hosts file
./bin/mw docker hosts add

# Create
./bin/mw docker mediawiki create
./bin/mw docker mediawiki create
./bin/mw docker mysql create

# Validate the basic stuff
./bin/mw docker docker-compose ps
./bin/mw docker env list

# Install, add host & check
./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
./bin/mw docker hosts add
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Suspend and resume and check the site is still there
./bin/mw docker mysql suspend
./bin/mw docker mysql resume
sleep 2
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Destroy and restart mysql, reinstalling mediawiki
./bin/mw docker mysql destroy
./bin/mw docker mysql create
./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Destroy it all
./bin/mw docker destroy
docker ps
docker ps | wc -l | grep -q "1"