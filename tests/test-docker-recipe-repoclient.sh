#!/usr/bin/env bash
#
# Test the wikibase-repoclient recipe end to end

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

# Set values needed for non-interactive runs
test_command_success ./bin/mw docker env clear
MWDIR=$(pwd)/.mediawiki
test_command_success ./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE ${MWDIR} --no-interaction

# Setup hosts
if ./bin/mw docker hosts writable --no-interaction; then
    test_command_success ./bin/mw docker hosts add --no-interaction
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./bin/mw docker hosts add --no-interaction
fi

# Validate and apply the recipe
test_command_success ./bin/mw dev recipe validate --name wikibase-repoclient
test_command_success ./bin/mw dev recipe --name wikibase-repoclient

# Make sure expected services are there
test_command ./bin/mw docker docker-compose ps "mediawiki-jobrunner"
test_command ./bin/mw docker docker-compose ps "mysql"

# Check expected hosts are present
test_file_contains "/etc/hosts" "default.mediawiki.local.wmftest.net"
test_file_contains "/etc/hosts" "client.mediawiki.local.wmftest.net"

# Check both sites respond
PORT=$(./bin/mw docker env get PORT)
test_wget http://default.mediawiki.local.wmftest.net:$PORT/wiki/Main_Page "MediaWiki"
test_wget http://client.mediawiki.local.wmftest.net:$PORT/wiki/Main_Page "hello"
