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
echo "*****************************************"
echo

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
# CI extracts the preparation job's snapshot before running this script. Keep it
# unchanged even if the week rolls over while the pipeline is running.
if [[ -z "${GITLAB_CI:-}" ]]; then
  source "$SCRIPT_DIR/cache-mediawiki.sh"
fi

# Create a fresh LocalSettings.php file
rm -f .mediawiki/LocalSettings.php
echo "<?php" >> .mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> .mediawiki/LocalSettings.php
echo "error_reporting(error_reporting() & ~(E_WARNING | E_DEPRECATED));" >> .mediawiki/LocalSettings.php

set +e
