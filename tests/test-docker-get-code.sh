#!/usr/bin/env bash
#
# Test the get-code command
# The other tests use a cached directory for MediaWiki code
# This test actually fetches code from the internet

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh

export MWCLI_CONTEXT_TEST=1

# Check we can clone just MediaWiki using a shallow clone and github
# TODO check remote used was really github?
# TODO check remote changes to gerrit!
# TODO check they are shallow clones...
TEST_DIR=$(mktemp -d)
TEST_TIMESTAMP=$(date +%s)
test_command_success ./bin/mw docker --context test-${TEST_TIMESTAMP}-just-core env set MEDIAWIKI_VOLUMES_CODE ${TEST_DIR}
test_command_success ./bin/mw docker --context test-${TEST_TIMESTAMP}-just-core mediawiki get-code --no-interaction --core --shallow --use-github --gerrit-interaction-type http
test_command_success cat ${TEST_DIR}/index.php
test_command sh -c "ls -lahrt ${TEST_DIR}/extensions | grep drwx | wc -l" 2
test_command sh -c "ls -lahrt ${TEST_DIR}/skins | grep drwx | wc -l" 2

# Check we can choose a few skins and extensions too
TEST_DIR=$(mktemp -d)
test_command_success ./bin/mw docker --context test-${TEST_TIMESTAMP}-core-plus env set MEDIAWIKI_VOLUMES_CODE ${TEST_DIR}
test_command_success ./bin/mw docker --context test-${TEST_TIMESTAMP}-core-plus mediawiki get-code --no-interaction --core --skin Vector --skin Timeless --extension Nuke --extension Cognate --shallow --gerrit-interaction-type http
test_command_success cat ${TEST_DIR}/index.php
test_command sh -c "ls -lahrt ${TEST_DIR}/extensions | grep drwx | wc -l" 4
test_command ls -lahrt ${TEST_DIR}/extensions "Nuke"
test_command ls -lahrt ${TEST_DIR}/extensions "Cognate"
test_command sh -c "ls -lahrt ${TEST_DIR}/skins | grep drwx | wc -l" 4
test_command ls -lahrt ${TEST_DIR}/skins "Vector"
test_command ls -lahrt ${TEST_DIR}/skins "Timeless"

# TODO check non shallow clone
# TODO check --gerrit-username
# TODO check --gerrit-interaction-type