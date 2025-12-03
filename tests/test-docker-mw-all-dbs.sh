#!/usr/bin/env bash
#
# This test installs all of the availible databases side by side
# making sure that all of the sites work on initial setup

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh
source $SCRIPT_DIR/pretest-mediawiki.sh

export MWCLI_CONTEXT_TEST=1

function finish {
    cd $SCRIPT_DIR/..

    # Show it all
    docker ps

    # Destroy it all
    test_command_success ./bin/mw docker destroy --no-interaction

    # Clean up & make sure no services are running
    test_docker_ps_service_count 0
    if ./bin/mw docker hosts writable --no-interaction; then
        test_command_success ./bin/mw docker hosts remove --no-interaction
    else
        echo "sudo needed for hosts file modification!"
        test_command_success sudo -E ./bin/mw docker hosts remove --no-interaction
    fi
    test_command_success ./bin/mw docker env clear --no-interaction
}

# Handle FINISH=1 environment variable
_handle_finish_if_needed

trap _finish_wrapper EXIT

# Change into the mediawiki directory, so we can auto detect the code!
cd ./.mediawiki

test_command_success ./../bin/mw docker env clear

# Setup the default hosts in hosts file
if ./../bin/mw docker hosts writable --no-interaction; then
    test_command_success ./../bin/mw docker hosts add --no-interaction
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./../bin/mw docker hosts add --no-interaction
fi

# Create, from the mediawiki dir, to allow --no-interaction to detect the existing mediawiki directory, setting the volume path
test_command_success ./../bin/mw docker mediawiki create --no-interaction
cd ./..

# Get the port in use
PORT=$(./bin/mw docker env get PORT)

# Make sure a site is running and not connected to a db
test_wget http://default.mediawiki.mwdd.localhost:$PORT "Could not find a running database for the database name"

# Turn on all of the services
test_command_success ./bin/mw docker mysql-replica create
test_command_success ./bin/mw docker postgres create
test_command_success ./bin/mw docker postgres create
test_command_success ./bin/mw docker phpmyadmin create
test_command_success ./bin/mw docker adminer create

# Install everything (mysql, postgres, sqlite)
test_command_success ./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql
test_command_success ./bin/mw docker mediawiki install --dbname postgreswiki --dbtype postgres
# This is currently disabled, as Warning: you have SQLite 3.27.2, which is lower than minimum required version 3.31.0. SQLite will be unavailable.
# This is due to still using an older mediawiki image due to https://phabricator.wikimedia.org/T388411
# test_command_success ./bin/mw docker mediawiki install --dbtype sqlite

# Test foreachwiki
test_command_success ./bin/mw docker mediawiki foreachwiki showSiteStats.php
test_command_success ./bin/mw docker mediawiki foreachwiki sql.php -- --query 'SELECT 1'

# Make sure mediawiki exec works for alternative db name
# Commented out 03/05/2024 as these not longer outputs a nice error https://phabricator.wikimedia.org/P61819
# test_command_success ./bin/mw docker mediawiki exec -- MW_DB=mysqlwiki composer phpunit tests/phpunit/unit/includes/xml/XmlTest.php | grep 'OK '
#test_command_success ./bin/mw docker mediawiki exec -- MW_DB=ddsadsadsaefault composer phpunit tests/phpunit/unit/includes/xml/XmlTest.php | grep 'Unable to find database'

# Update the hosts file as we used new wiki names
if ./bin/mw docker hosts writable; then
    test_command_success ./bin/mw docker hosts add
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./bin/mw docker hosts add
fi

# Check the DB tools (phpmyadmin, adminer)
test_wget http://phpmyadmin.mwdd.localhost:$PORT "Open new phpMyAdmin window"
test_wget http://adminer.mwdd.localhost:$PORT "Login - Adminer"

# And check the installed sites (mysql, postgres, sqlite)
# test_wget http://default.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"
test_wget http://postgreswiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"
test_wget http://mysqlwiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"

# Make sure the expected number of services appear
test_docker_ps_service_count 9

# Try other DB related commands
test_command_success ./bin/mw docker postgres stop
test_command_success ./bin/mw docker postgres start
sleep 1
test_command_success ./bin/mw docker postgres exec echo foo
test_command_success ./bin/mw docker mysql-replica stop
test_command_success ./bin/mw docker mysql-replica start
sleep 1
test_command_success ./bin/mw docker mysql-replica exec echo foo
# TODO test the mysql and replica "mysql" commands (cli)
test_command_success ./bin/mw docker mysql exec echo foo