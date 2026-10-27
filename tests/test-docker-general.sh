#!/usr/bin/env bash
#
# Tests general docker commands

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh
source "$SCRIPT_DIR/guide-examples.sh"
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

test_command_success ./bin/mw dev env clear --no-interaction

# Also test using --context as well in a few ways with valid commands
test_command_success ./bin/mw dev env list --no-interaction --context=anotherone
test_command_success ./bin/mw dev --context=anotherone env list --no-interaction
test_command_success ./bin/mw dev --context=anotherone env list --no-interaction --help
test_command_success ./bin/mw dev --context anotherone env list --no-interaction --help
test_command_success ./bin/mw dev -c anotherone env list --no-interaction --help

# Run this integration test using a non standard port, unlikley to conflict, to make sure it works
test_command_success ./bin/mw dev env set PORT 6194
# And already fill in the location of mediawiki
MWDIR=$(pwd)/.mediawiki
test_command_success ./bin/mw dev env set MEDIAWIKI_VOLUMES_CODE ${MWDIR}

test_command_success ./bin/mw dev hosts show --no-interaction

# Setup the default hosts in hosts file
if ./bin/mw dev hosts writable --no-interaction; then
    test_command_success ./bin/mw dev hosts add --no-interaction
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./bin/mw dev hosts add --no-interaction
fi

test_command_success ./bin/mw dev hosts show --no-interaction

# Create
test_command_success ./bin/mw dev mediawiki create

# Ensure custom compose file/service functionality works
CUSTOM_FILE=$(./bin/mw dev custom --name custom-two where)
cat > "${CUSTOM_FILE}" << 'EOF'
services:
    custom-two:
        image: "alpine:3.20"
        command: ["sleep", "infinity"]
        security_opt:
            - label:disable
        dns:
            - ${NETWORK_SUBNET_PREFIX}.10
        networks:
            - dps
EOF
test_file_contains "${CUSTOM_FILE}" "custom-two"
test_command_success ./bin/mw dev custom --name custom-two create
test_command ./bin/mw dev docker-compose ps "custom-two"
test_command_success ./bin/mw dev custom --name custom-two stop
test_command_success ./bin/mw dev custom --name custom-two start
test_command_success ./bin/mw dev custom --name custom-two destroy

# Get the port in use
PORT=$(./bin/mw dev env get PORT)

# Make sure that exec generally works as expected
./bin/mw dev mediawiki exec -- FOO=bar env | grep FOO

# Validate the basic stuff
test_command_success ./bin/mw dev docker-compose ps
test_command_success ./bin/mw dev env list

test_wget http://default.mediawiki.local.wmftest.net:$PORT "Could not find a running database for the database name"

# Install mysql & check
# These used to use sqlite, but due to https://phabricator.wikimedia.org/T330940 mysql is needed for the browser tests to not error
test_command_success ./bin/mw dev mysql create
test_command_success ./bin/mw dev mediawiki install --dbtype mysql
test_wget http://default.mediawiki.local.wmftest.net:$PORT "MediaWiki has been installed"
# The local catch-all uses the configured default wiki, not the hostname as a database name.
test_wget "http://localhost:$PORT/w/api.php?action=query&meta=siteinfo&format=json" '"sitename":"Dev wiki - default"'

# Set the default dbname to something else, restarting the container
test_command_success ./bin/mw dev env set MEDIAWIKI_DEFAULT_DBNAME second
test_command_success ./bin/mw dev mediawiki create
# And install another site
test_command_success ./bin/mw dev mediawiki install --dbtype mysql
# Update the hosts file again to include the new site
if ./bin/mw dev hosts writable --no-interaction; then
    test_command_success ./bin/mw dev hosts add --no-interaction
else
    echo "sudo needed for hosts file modification!"
    test_command_success sudo -E ./bin/mw dev hosts add --no-interaction
fi
test_wget http://second.mediawiki.local.wmftest.net:$PORT "MediaWiki has been installed"
test_wget "http://localhost:$PORT/w/api.php?action=query&meta=siteinfo&format=json" '"sitename":"Dev wiki - second"'
test_wget "http://default.mediawiki.local.wmftest.net:$PORT/w/api.php?action=query&meta=siteinfo&format=json" '"sitename":"Dev wiki - default"'

# Make sure that maintenance scripts run for the current default wiki dbname
test_command ./bin/mw dev mediawiki mwscript "requires at least 1 arg(s), only received 0"
test_command_success ./bin/mw dev mediawiki mwscript Version # Runs on second
test_command_success ./bin/mw dev mediawiki mwscript MW_DB=default Version # Runs on default
test_command_success ./bin/mw dev mediawiki mwscript Version -- --wiki=default # Runs on default
# If we set to some random dbanme we get errors
test_command_success ./bin/mw dev env set MEDIAWIKI_DEFAULT_DBNAME nonexistent
test_command_success ./bin/mw dev mediawiki create
test_command ./bin/mw dev mediawiki mwscript Version "Unable to find database"
# An reset eveyrthing to normal, so "default" is used
test_command_success ./bin/mw dev env delete MEDIAWIKI_DEFAULT_DBNAME nonexistent
test_command_success ./bin/mw dev mediawiki create

# Check the doctor
test_command_success ./bin/mw dev mediawiki doctor

# Make sure the shellbox service commands work
# TODO text exec command
test_command_success ./bin/mw dev shellbox media create
test_command_success ./bin/mw dev shellbox media exec echo foo
test_command ./bin/mw dev shellbox media exec echo foo "foo"
test_command_success ./bin/mw dev shellbox media stop
test_command_success ./bin/mw dev shellbox media start
test_command_success ./bin/mw dev shellbox media destroy
# Internally these all work the same, so this tests "them all"
# SUGGEST cmd: mw dev shellbox php-rpc: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox php-rpc create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox php-rpc destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox php-rpc exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox php-rpc start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox php-rpc stop: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox score: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox score create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox score destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox score exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox score start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox score stop: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox syntaxhighlight: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox syntaxhighlight create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox syntaxhighlight destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox syntaxhighlight exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox syntaxhighlight start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox syntaxhighlight stop: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox timeline: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox timeline create: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox timeline destroy: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox timeline exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox timeline start: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw dev shellbox timeline stop: (end-to-end-test) End to end tests are suggested, none found

# Make sure `docker update` works as expected
test_command_success ./bin/mw dev shellbox media create
test_command_success ./bin/mw dev shellbox media stop
test_command_success ./bin/mw dev update
# TODO could make sure shellbox media is still stopped after the update..

# Validate the Xdebug guide against the real PHP service.
test_xdebug_settings "$(pwd)/bin/mw"

# Change an already running gateway, not just its initial .env value.
test_http_port_change "$(pwd)/bin/mw" 6195
PORT=$(./bin/mw dev env get PORT)
test_wget_eventually_contains "http://default.mediawiki.local.wmftest.net:$PORT/wiki/Main_Page" "MediaWiki has been installed" 60 1

# cd to mediawiki
cd .mediawiki

# composer: Make sure a command works in root of the repo
test_command ./../bin/mw dev mediawiki composer home "https://www.mediawiki.org/"

# exec: Make sure a command works in the root of the repo
test_command ./../bin/mw dev mediawiki exec ls "api.php"

# exec phpunit: Make sure using exec to run phpunit things works
# Disabled 03/05/2024 as it was failing for unknown reasons... https://gitlab.wikimedia.org/repos/releng/cli/-/jobs/250710
# PHP Fatal error:  Uncaught TypeError: implode(): Argument #1 ($pieces) must be of type array, string given in /var/www/html/w/tests/phpunit/getPHPUnitExtensionsAndSkins.php:44
# test_command "./../bin/mw dev mediawiki exec -- composer phpunit tests/phpunit/unit/includes/installer/PingbackTest.php" "OK "

# fresh: Run the guide example and verify its recording, using the existing browser test.
test_fresh_recording "$(cd .. && pwd)/bin/mw"

# quibble: Make sure a quibble works
test_command_success ./../bin/mw dev mediawiki quibble quibble -- --help
test_command ./../bin/mw dev mediawiki quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --command \"ls\" "index.php"

# jobrunner: make sure the jobrunner starts and can run jobs
test_command_success ./../bin/mw dev mediawiki jobrunner create
test_command_success ./../bin/mw dev mediawiki jobrunner add-site default
test_command_success bash -c "./../bin/mw wiki page put --wiki http://default.mediawiki.local.wmftest.net:$PORT/w/api.php --user admin --password mwddpassword --title Testpage1 <<< 'Test content'"
test_command_success bash -c "./../bin/mw wiki page put --wiki http://default.mediawiki.local.wmftest.net:$PORT/w/api.php --user admin --password mwddpassword --title Testpage2 <<< 'Test content'"
# Job execution is asynchronous. An old empty-queue message is not enough:
# wait for both test pages' jobs to complete and then the queue to drain.
# The assertions below still print the logs if the wait times out.
jobrunner_ready=0
for attempt in {1..60}; do
    if ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner 2>&1 | awk '
        /title=Testpage1([[:space:]]|$)/ && / good[[:space:]]*$/ { first_done = 1 }
        /title=Testpage2([[:space:]]|$)/ && / good[[:space:]]*$/ { second_done = 1 }
        first_done && second_done && / Job queue is empty/ { drained = 1 }
        END { exit !drained }
    '; then
        jobrunner_ready=1
        break
    fi
    sleep 1
done
# We expect to see all of this output in the logs for the job runner...
test_command ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner " Running jobs for default"
test_command ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner " Job queue is empty"
test_command ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner " STARTING"
test_command ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner " title=Testpage1"
test_command ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner " good"
if [ "$jobrunner_ready" -ne 1 ]; then
    echo "FAIL: test page jobs did not complete and drain the queue within 60 attempts"
    ./../bin/mw dev docker-compose logs -- --tail all mediawiki-jobrunner
    exit 1
fi

# image get/set/reset alters env
test_command ./../bin/mw dev env has MEDIAWIKI_IMAGE "var does not exist"
test_command_success ./../bin/mw dev mediawiki image set foo
test_command ./../bin/mw dev mediawiki image get "foo"
test_command ./../bin/mw dev env has MEDIAWIKI_IMAGE "var exists"
test_command_success ./../bin/mw dev mediawiki image reset
test_command ./../bin/mw dev env has MEDIAWIKI_IMAGE "var does not exist"

# cd to Vector
cd skins/Vector

# composer: Make sure a command works from the Vector directory
test_command ./../../../bin/mw dev mediawiki composer home "http://gerrit.wikimedia.org/g/mediawiki/skins/Vector"
# exec: Make sure a command works from the Vector directory
test_command ./../../../bin/mw dev mediawiki exec ls "skin.json"

# gerrit dotgitreview project
test_command ./../../../bin/mw gerrit dotgitreview project "mediawiki/skins/Vector"
