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
    echo "---------------------------------------"
    echo "Finishing up and cleaning up tests..."
    echo "---------------------------------------"
    cd $SCRIPT_DIR/..

    # Show it all
    docker ps

    # Destroy it all
    test_command_success ./bin/mw dev destroy --no-interaction

    # Clean up & make sure no services are running
    test_docker_ps_service_count 0
    if ./bin/mw dev hosts writable --no-interaction; then
        test_command_success ./bin/mw dev hosts remove --no-interaction
    else
        echo "sudo needed for hosts file modification!"
        test_command_success sudo -E ./bin/mw dev hosts remove --no-interaction
    fi
    test_command_success ./bin/mw dev env clear --no-interaction
}

# Handle FINISH=1 environment variable
_handle_finish_if_needed

trap _finish_wrapper EXIT

# Change into the mediawiki directory, so we can auto detect the code!
cd ./.mediawiki

test_command_success ./../bin/mw dev env clear

# Setup the default hosts in hosts file
if ./../bin/mw dev hosts writable --no-interaction; then
    test_command_success ./../bin/mw dev hosts add --no-interaction
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./../bin/mw dev hosts add --no-interaction
fi

# Create, from the mediawiki dir, to allow --no-interaction to detect the existing mediawiki directory, setting the volume path
test_command_success ./../bin/mw dev mediawiki create --no-interaction
cd ./..

# Get the port in use
PORT=$(./bin/mw dev env get PORT)

# Make sure a site is running and not connected to a db
test_wget_eventually_contains http://default.mediawiki.local.wmftest.net:$PORT "Could not find a running database for the database name" 60 1

# Turn on all of the services
test_command_success ./bin/mw dev mysql-replica create
test_command_success ./bin/mw dev postgres create
test_command_success ./bin/mw dev postgres create
test_command_success ./bin/mw dev phpmyadmin create
test_command_success ./bin/mw dev adminer create

# Install everything (mysql, postgres, sqlite)
test_command_success ./bin/mw dev mediawiki install --dbname mysqlwiki --dbtype mysql
test_command_success ./bin/mw dev mediawiki install --dbname postgreswiki --dbtype postgres
# foreachwiki's read-only maintenance commands may use the MySQL replica. Wait
# for the newly installed MySQL schema to replicate before exercising it.
replica_wait_attempt=0
while [ "$replica_wait_attempt" -lt 60 ]; do
    if replica_site_stats_table=$(./bin/mw dev mysql-replica exec -- mysql -uroot -ptoor -Nse "SHOW TABLES FROM mysqlwiki LIKE 'site_stats'" 2>/dev/null | tr -d '\r') && [ "$replica_site_stats_table" = "site_stats" ]; then
        break
    fi
    replica_wait_attempt=$((replica_wait_attempt + 1))
    sleep 1
done
if [ "$replica_site_stats_table" != "site_stats" ]; then
    echo "FAIL: mysqlwiki.site_stats did not replicate to mysql-replica within 60 seconds"
    exit 1
fi
echo "PASS: mysqlwiki.site_stats replicated to mysql-replica"

# This is currently disabled, as Warning: you have SQLite 3.27.2, which is lower than minimum required version 3.31.0. SQLite will be unavailable.
# This is due to still using an older mediawiki image due to https://phabricator.wikimedia.org/T388411
# test_command_success ./bin/mw dev mediawiki install --dbtype sqlite

# Test foreachwiki
test_command_success ./bin/mw dev mediawiki foreachwiki showSiteStats.php
test_command_success ./bin/mw dev mediawiki foreachwiki sql.php -- --query 'SELECT 1'

# Make sure mediawiki exec works for alternative db name
# Commented out 03/05/2024 as these not longer outputs a nice error https://phabricator.wikimedia.org/P61819
# test_command_success ./bin/mw dev mediawiki exec -- MW_DB=mysqlwiki composer phpunit tests/phpunit/unit/includes/xml/XmlTest.php | grep 'OK '
#test_command_success ./bin/mw dev mediawiki exec -- MW_DB=ddsadsadsaefault composer phpunit tests/phpunit/unit/includes/xml/XmlTest.php | grep 'Unable to find database'

# Update the hosts file as we used new wiki names
if ./bin/mw dev hosts writable; then
    test_command_success ./bin/mw dev hosts add
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./bin/mw dev hosts add
fi

# Check the DB tools (phpmyadmin, adminer)
test_wget http://phpmyadmin.local.wmftest.net:$PORT "Open new phpMyAdmin window"
test_wget http://adminer.local.wmftest.net:$PORT "Login - Adminer"
test_http_status http://adminer.local.wmftest.net:$PORT/ 200
test_http_status http://127.0.0.1:$PORT/proxy/adminer/ 401
test_http_status http://127.0.0.1:$PORT/proxy/adminer/ 401 0 adminer.local.wmftest.net
test_http_status http://127.0.0.1:$PORT/proxy/adminer/ 200 1
test_http_status http://127.0.0.1:$PORT/ 401
test_gateway_https_session http://127.0.0.1:$PORT

# Requests from MediaWiki must resolve through DPS and reach the same listener
# port as host requests, without requiring the public gateway login.
test_command_success ./bin/mw dev mediawiki exec -- curl --noproxy '*' --connect-timeout 5 --max-time 10 -fsS -o /dev/null http://adminer.local.wmftest.net:$PORT/
test_command_success ./bin/mw dev mediawiki exec -- curl --noproxy '*' --connect-timeout 5 --max-time 10 -fsS -o /dev/null http://mysqlwiki.mediawiki.local.wmftest.net:$PORT/wiki/Main_Page

# And check the installed sites (mysql, postgres, sqlite)
# test_wget http://default.mediawiki.local.wmftest.net:$PORT "MediaWiki has been installed"
test_wget http://postgreswiki.mediawiki.local.wmftest.net:$PORT "MediaWiki has been installed"
test_wget http://mysqlwiki.mediawiki.local.wmftest.net:$PORT "MediaWiki has been installed"
# Follow the gateway redirect; local HTTP must not redirect to an HTTPS listener.
test_wget http://127.0.0.1:$PORT/proxy/mysqlwiki/ "MediaWiki has been installed"
# Assets must retain the wiki prefix and load from the selected wiki.
test_wget http://127.0.0.1:$PORT/proxy/mysqlwiki/wiki/Main_Page 'href="/proxy/mysqlwiki/w/load.php'
test_wget "http://127.0.0.1:$PORT/proxy/mysqlwiki/w/load.php?lang=en&modules=skins.vector.styles&only=styles&skin=vector-2022" '.mw'
test_wget "http://127.0.0.1:$PORT/proxy/mysqlwiki/w/load.php?lang=en&modules=startup&only=scripts&raw=1&skin=vector-2022" 'mw.loader'

# Make sure the expected number of services appear
test_docker_ps_service_count 12

# Try other DB related commands
test_command_success ./bin/mw dev postgres stop
test_command_success ./bin/mw dev postgres start
sleep 1
test_command_success ./bin/mw dev postgres exec echo foo
test_command_success ./bin/mw dev mysql-replica stop
test_command_success ./bin/mw dev mysql-replica start
sleep 1
test_command_success ./bin/mw dev mysql-replica exec echo foo
# TODO test the mysql and replica "mysql" commands (cli)
test_command_success ./bin/mw dev mysql exec echo foo
