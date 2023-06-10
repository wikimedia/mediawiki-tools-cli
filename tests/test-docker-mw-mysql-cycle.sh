#!/usr/bin/env bash
#
# This test creates a site using a mysql backend, makes sure it works.
# It suspends everything, restarting it, checking it is up
# before destroying it and creating it again.

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

# Set some correct values so we don't get asked
test_command_success "./bin/mw docker env clear"
MWDIR=$(pwd)/.mediawiki
test_command_success "./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE ${MWDIR} --no-interaction"

cat /etc/hosts

# Setup the default hosts in hosts file & clear previous env vars
if ./bin/mw docker hosts writable --no-interaction; then
    test_command_success "./bin/mw docker hosts add --no-interaction"
else
    echo "sudo needed for hosts file modification!"
    test_command_success "sudo -E ./bin/mw docker hosts add --no-interaction"
fi

cat /etc/hosts

# Create with  --no-interaction so a port is claimed
test_command_success "./bin/mw docker mediawiki create"
test_command_success "./bin/mw docker mysql create"

# Get the port in use
PORT=$(./bin/mw docker env get PORT)

# Install, add host & check
test_command_success "./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql"
if ./bin/mw docker hosts writable; then
    test_command_success "./bin/mw docker hosts add"
else
    echo "sudo needed for hosts file modification!"
    test_command_success "sudo -E ./bin/mw docker hosts add"
fi
test_file_contains "/etc/hosts" "mysqlwiki.mediawiki.mwdd.localhost"
test_curl http://mysqlwiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"

# Stop and start and check the site is still there
test_command_success "./bin/mw docker mysql stop"
test_command_success "./bin/mw docker mysql start"
sleep 5
test_curl http://mysqlwiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"

# Destroy and restart mysql, reinstalling mediawiki
test_command_success "./bin/mw docker mysql destroy"
test_command_success "./bin/mw docker mysql create"
test_command_success "./bin/mw docker mediawiki install --dbname mysqlwiki --dbtype mysql"
test_curl http://mysqlwiki.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"
