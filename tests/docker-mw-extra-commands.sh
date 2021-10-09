#!/usr/bin/env bash

set -e # Fail on errors
set -x # Output commands

# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT

# Setup the default hosts in hosts file
./bin/mw docker hosts add

# Run this integration test using a non standard port
./bin/mw docker env set PORT 9191
# And already fill in the location of mediawiki
./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
# So we should get no prompts, even though we don't pass --no-interaction

# Create
./bin/mw docker mediawiki create

# Make sure that exec generally works as expected
./bin/mw docker mediawiki exec -- FOO=bar env | grep FOO

# Validate the basic stuff
./bin/mw docker docker-compose ps
./bin/mw docker env list
CURL=$(curl -s -L -N http://default.mediawiki.mwdd.localhost:9191) && echo $CURL && echo $CURL | grep -q "Is your database running and wiki database created"

# Install sqlite & check
./bin/mw docker mediawiki install --dbtype sqlite
CURL=$(curl -s -L -N http://default.mediawiki.mwdd.localhost:9191) && echo $CURL && echo $CURL | grep -q "MediaWiki has been installed"

# docker-compose: Make sure it appears to work
./bin/mw docker docker-compose ps -- --services | grep -q "mediawiki"

# cd to mediawiki
cd mediawiki

# composer: Make sure a command works in root of the repo
./../bin/mw docker mediawiki composer home | grep -q "https://www.mediawiki.org/"

# exec: Make sure a command works in the root of the repo
./../bin/mw docker mediawiki exec ls | grep -q "api.php"

# exec phpunit: Make sure using exec to run phpunit things works
./../bin/mw docker mediawiki exec -- composer phpunit tests/phpunit/unit/includes/PingbackTest.php
./../bin/mw docker mediawiki exec -- composer phpunit tests/phpunit/unit/includes/PingbackTest.php | grep -q "OK "

# fresh: Make sue a basic browser test works
./../bin/mw docker mediawiki fresh npm run selenium-test -- -- --spec tests/selenium/specs/page.js

# quibble: Make sure a quibble works
./../bin/mw docker mediawiki quibble quibble -- --help
./../bin/mw docker mediawiki quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --command "ls"

# cd to Vector
cd skins/Vector

# composer: Make sure a command works from the Vector directory
./../../../bin/mw docker mediawiki composer home | grep -q "http://gerrit.wikimedia.org/g/mediawiki/skins/Vector"
# exec: Make sure a command works from the Vector directory
./../../../bin/mw docker mediawiki exec ls | grep -q "skin.json"

# gerrit project current
./../../../bin/mw gerrit project current | grep -q "mediawiki/skins/Vector"

# cd back again
cd ./../../../

# Destroy it all
./bin/mw docker destroy

# Remove hosts
./bin/mw docker hosts delete

# And make sure only 1 exists after
docker ps
docker ps | wc -l | grep -q "1"