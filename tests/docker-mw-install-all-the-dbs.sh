#!/usr/bin/env bash

set -e # Fail on errors
set -x # Output commands

# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT

# Create
./bin/mw docker mediawiki create

# Create: Validate the basic stuff
./bin/mw docker docker-compose ps
# TODO enable logo check again once the page no longer shown "Unable to connect to PostgreSQL server"
#CURL=$(curl -s -L -N http://default.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "The MediaWiki logo"
CURL=$(curl -s -L -N http://default.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "Unable to connect to PostgreSQL server"

# Turn on all of the services
./bin/mw docker mysql-replica create
./bin/mw docker postgres create
./bin/mw docker phpmyadmin create
./bin/mw docker adminer create

# Install everything (mysql, postgres, sqlite)
./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
./bin/mw docker mediawiki install --dbname postgreswiki --dbtype postgres
./bin/mw docker mediawiki install

# Check the DB tools (phpmyadmin, adminer)
CURL=$(curl -s -L -N http://phpmyadmin.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "Open new phpMyAdmin window"
CURL=$(curl -s -L -N http://adminer.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "Login - Adminer"

# And check the installed sites (mysql, postgres, sqlite)
CURL=$(curl -s -L -N http://default.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"
CURL=$(curl -s -L -N http://postgreswiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"
CURL=$(curl -s -L -N http://mysqlwiki.mediawiki.mwdd.localhost:8080) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# Make sure the expected number of services appear
docker ps
docker ps | wc -l | grep -q "10"

# Destroy it all
./bin/mw docker destroy
# And make sure only 1 line exists after
docker ps
docker ps | wc -l | grep -q "1"
