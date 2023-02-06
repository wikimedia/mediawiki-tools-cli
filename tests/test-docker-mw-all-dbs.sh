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
    test_command_success "./bin/mw docker destroy --no-interaction"

    # Clean up & make sure no services are running
    test_docker_ps_service_count 0
    if ./bin/mw docker hosts writable --no-interaction; then
        test_command_success "./bin/mw docker hosts remove --no-interaction"
    else
        echo "sudo needed for hosts file modification!"
        test_command_success "sudo -E ./bin/mw docker hosts remove --no-interaction"
    fi
    test_command_success "./bin/mw docker env clear --no-interaction"
}
trap finish EXIT

# Change into the mediawiki directory, so we can auto detect the code!
cd ./.mediawiki

test_command_success "./../bin/mw docker env clear"

# Setup the default hosts in hosts file
if ./../bin/mw docker hosts writable --no-interaction; then
    test_command_success "./../bin/mw docker hosts add --no-interaction"
else
    echo "sudo needed for hosts file modification!"
    test_command_success "sudo -E ./../bin/mw docker hosts add --no-interaction"
fi

# Create, from the mediawiki dir, to allow --no-interaction to detect the existing mediawiki directory, setting the volume path
test_command_success "./../bin/mw docker mediawiki create --no-interaction"
cd ./..

# Get the port in use
PORT=$(./bin/mw docker env get PORT)

# Make sure a site is running and not connected to a db
test_curl http://default.mediawiki.mwdd.localhost:$PORT "Could not find a running database for the database name"

# Turn on all of the services
test_command_success "./bin/mw docker mysql-replica create"
test_command_success "./bin/mw docker postgres create"
test_command_success "./bin/mw docker postgres create"
test_command_success "./bin/mw docker phpmyadmin create"
test_command_success "./bin/mw docker adminer create"

# Install everything (mysql, postgres, sqlite)
test_command_success "./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql"
test_command_success "./bin/mw docker mediawiki install --dbname postgreswiki --dbtype postgres"
test_command_success "./bin/mw docker mediawiki install --dbtype sqlite"

# Make sure mediawiki exec works for alternative db name
test_command_success "./bin/mw docker mediawiki exec -- MW_DB=mysqlwiki composer phpunit tests/phpunit/unit/includes/XmlTest.php | grep 'seconds'"
# And doesnt work with a non existant name
test_command_success "./bin/mw docker mediawiki exec -- MW_DB=ddsadsadsaefault composer phpunit tests/phpunit/unit/includes/XmlTest.php | grep 'Unable to find database'"

# Update the hosts file as we used new wiki names
if ./bin/mw docker hosts writable; then
    test_command_success "./bin/mw docker hosts add"
else
    echo "sudo needed for hosts file modification!"
    test_command_success "sudo -E ./bin/mw docker hosts add"
fi

# Check the DB tools (phpmyadmin, adminer)
test_curl http://phpmyadmin.mwdd.localhost:$PORT "Open new phpMyAdmin window"
test_curl http://adminer.mwdd.localhost:$PORT "Login - Adminer"

# And check the installed sites (mysql, postgres, sqlite)
test_curl http://default.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"
test_curl http://postgreswiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"
test_curl http://mysqlwiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"

# Make sure the expected number of services appear
test_docker_ps_service_count 9
