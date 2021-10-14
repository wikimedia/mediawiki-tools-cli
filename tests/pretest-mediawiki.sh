#!/usr/bin/env bash
#
# Setup a .mediawiki directory for testing
# This should not use the mwcli itself!

set -e # Fail on errors

# Output some useful docker version information
echo "Outputing some useful debug infomation as part of tests..."
echo "*****************************************"
uname -a
echo "I am:" $(whoami)
docker --version
docker-compose version
# Output CLI version
./bin/mw version
echo "*****************************************"
echo

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/cache-mediawiki.sh

# Create a fresh LocalSettings.php file
rm -f .mediawiki/LocalSettings.php
echo "<?php" >> .mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> .mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> .mediawiki/LocalSettings.php

set +e