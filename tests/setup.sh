#!/usr/bin/env bash

set -e # Fail on errors
set -x # Output commands

# Output some useful docker version information
docker --version
docker-compose version

# Output CLI version
./bin/mw version

# Setup things that otherwise need user input
./bin/mw docker env set PORT 8080
./bin/mw docker env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
# And output their values
./bin/mw docker env list
cat $(./bin/mw docker env where)

# Create a fresh LocalSettings.php file
rm -f mediawiki/LocalSettings.php
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php