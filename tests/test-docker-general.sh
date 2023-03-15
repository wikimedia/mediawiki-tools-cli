#!/usr/bin/env bash
#
# Tests general docker commands

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

test_command_success "./bin/mw docker env clear --no-interaction"

# Run this integration test using a non standard port, unlikley to conflict, to make sure it works
test_command_success "./bin/mw docker env set PORT 6194"
# And already fill in the location of mediawiki
MWDIR=$(pwd)/.mediawiki
test_command_success "./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE ${MWDIR}"

# Setup the default hosts in hosts file
if ./bin/mw docker hosts writable --no-interaction; then
    test_command_success "./bin/mw docker hosts add --no-interaction"
else
    echo "sudo needed for hosts file modification!"
    test_command_success "sudo -E ./bin/mw docker hosts add --no-interaction"
fi

# Create
test_command_success "./bin/mw docker mediawiki create"

# Get the port in use
PORT=$(./bin/mw docker env get PORT)

# Make sure that exec generally works as expected
./bin/mw docker mediawiki exec -- FOO=bar env | grep FOO

# Validate the basic stuff
test_command_success "./bin/mw docker docker-compose ps"
test_command_success "./bin/mw docker env list"

test_curl http://default.mediawiki.mwdd.localhost:$PORT "Could not find a running database for the database name"

# Install mysql & check
# These used to use sqlite, but due to https://phabricator.wikimedia.org/T330940 mysql is needed for the browser tests to not error
test_command_success "./bin/mw docker mysql create"
test_command_success "./bin/mw docker mediawiki install --dbtype mysql"
test_curl http://default.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"

# Set the default dbname to something else, restarting the container
test_command_success "./bin/mw docker env set MEDIAWIKI_DEFAULT_DBNAME second"
test_command_success "./bin/mw docker mediawiki create"
# And install another site
test_command_success "./bin/mw docker mediawiki install --dbtype mysql"
# Update the hosts file again to include the new site
if ./bin/mw docker hosts writable --no-interaction; then
    test_command_success "./bin/mw docker hosts add --no-interaction"
else
    echo "sudo needed for hosts file modification!"
    test_command_success "sudo -E ./bin/mw docker hosts add --no-interaction"
fi
test_curl http://second.mediawiki.mwdd.localhost:$PORT "MediaWiki has been installed"

# Make sure that maintenance scripts run for the current default wiki dbname
test_command "./bin/mw docker mediawiki mwscript" "Argument <script> is required!"
test_command_success "./bin/mw docker mediawiki mwscript version" # Runs on second
test_command_success "./bin/mw docker mediawiki mwscript MW_DB=default version" # Runs on default
test_command_success "./bin/mw docker mediawiki mwscript version -- --wiki=default" # Runs on default
# If we set to some random dbanme we get errors
test_command_success "./bin/mw docker env set MEDIAWIKI_DEFAULT_DBNAME nonexistent"
test_command_success "./bin/mw docker mediawiki create"
test_command "./bin/mw docker mediawiki mwscript version" "Unable to find database"
# An reset eveyrthing to normal, so "default" is used
test_command_success "./bin/mw docker env delete MEDIAWIKI_DEFAULT_DBNAME nonexistent"
test_command_success "./bin/mw docker mediawiki create"

# Check the doctor
test_command_success "./bin/mw docker mediawiki doctor"

# Make sure the shellbox service commands work
# TODO text exec command
test_command_success "./bin/mw docker shellbox media create"
test_command_success "./bin/mw docker shellbox media exec echo foo"
test_command "./bin/mw docker shellbox media exec echo foo" "foo"
test_command_success "./bin/mw docker shellbox media stop"
test_command_success "./bin/mw docker shellbox media start"
test_command_success "./bin/mw docker shellbox media destroy"
# Internally these all work the same, so this tests "them all"
# SUGGEST cmd: mw docker shellbox php-rpc: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox php-rpc create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox php-rpc destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox php-rpc exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox php-rpc start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox php-rpc stop: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox score: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox score create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox score destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox score exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox score start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox score stop: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox syntaxhighlight: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox syntaxhighlight create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox syntaxhighlight destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox syntaxhighlight exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox syntaxhighlight start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox syntaxhighlight stop: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox timeline: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox timeline create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox timeline destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox timeline exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox timeline start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw docker shellbox timeline stop: (end-to-end-test) End to end tests are suggested, none found

# cd to mediawiki
cd .mediawiki

# composer: Make sure a command works in root of the repo
test_command "./../bin/mw docker mediawiki composer home" "https://www.mediawiki.org/"

# exec: Make sure a command works in the root of the repo
test_command "./../bin/mw docker mediawiki exec ls" "api.php"

# exec phpunit: Make sure using exec to run phpunit things works
test_command "./../bin/mw docker mediawiki exec -- composer phpunit tests/phpunit/unit/includes/PingbackTest.php" "OK "

# fresh: Make sue a basic browser test works
test_command_success "./../bin/mw docker mediawiki fresh npm run selenium-test -- -- --spec tests/selenium/specs/page.js"

# quibble: Make sure a quibble works
test_command_success "./../bin/mw docker mediawiki quibble quibble -- --help"
test_command "./../bin/mw docker mediawiki quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --command \"ls\"" "index.php"

# jobrunner: make sure the jobrunner starts and can run jobs
test_command_success "./../bin/mw docker mediawiki jobrunner create"
test_command_success "./../bin/mw docker mediawiki jobrunner add-site default"
test_command_success "./../bin/mw wiki page put --wiki http://default.mediawiki.mwdd.localhost:$PORT/w/api.php --user admin --password mwddpassword --title 'Testpage1' <<< 'Test content'"
test_command_success "./../bin/mw wiki page put --wiki http://default.mediawiki.mwdd.localhost:$PORT/w/api.php --user admin --password mwddpassword --title 'Testpage2' <<< 'Test content'"
# We expect to see all of this output in the logs for the job runner...
test_command "./../bin/mw docker docker-compose logs -- --tail all mediawiki-jobrunner" " No sites to run jobs for"
test_command "./../bin/mw docker docker-compose logs -- --tail all mediawiki-jobrunner" " Running jobs for default"
test_command "./../bin/mw docker docker-compose logs -- --tail all mediawiki-jobrunner" " Job queue is empty"
test_command "./../bin/mw docker docker-compose logs -- --tail all mediawiki-jobrunner" " STARTING"
test_command "./../bin/mw docker docker-compose logs -- --tail all mediawiki-jobrunner" " title='Testpage1'"
test_command "./../bin/mw docker docker-compose logs -- --tail all mediawiki-jobrunner" " good"

# image get/set/reset alters env
test_command "./../bin/mw docker env has MEDIAWIKI_IMAGE" "var does not exist"
test_command_success "./../bin/mw docker mediawiki image set foo"
test_command "./../bin/mw docker mediawiki image get" "foo"
test_command "./../bin/mw docker env has MEDIAWIKI_IMAGE" "var exists"
test_command_success "./../bin/mw docker mediawiki image reset"
test_command "./../bin/mw docker env has MEDIAWIKI_IMAGE" "var does not exist"

# get the example skin using get-code
# Remove it both before and after incase it is left and to avoid it being left in CI caches
rm -rf ${MWDIR}/skins/Example
test_command_success "./../bin/mw docker mediawiki get-code --skin Example"
rm -rf ${MWDIR}/skins/Example

# cd to Vector
cd skins/Vector

# composer: Make sure a command works from the Vector directory
test_command "./../../../bin/mw docker mediawiki composer home" "http://gerrit.wikimedia.org/g/mediawiki/skins/Vector"
# exec: Make sure a command works from the Vector directory
test_command "./../../../bin/mw docker mediawiki exec ls" "skin.json"

# gerrit dotgitreview project
test_command "./../../../bin/mw gerrit dotgitreview project" "mediawiki/skins/Vector"
