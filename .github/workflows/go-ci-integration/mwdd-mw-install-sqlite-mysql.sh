#!/usr/bin/env bash

# Setup & Create
./mw mwdd env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
./mw mwdd create

# Validate the basic stuff
./mw mwdd docker-compose ps
./mw mwdd env list
cat ~/.mwcli/mwdd/default/.env
curl -s http://default.mediawiki.mwdd.localhost:8080 | grep -q "The MediaWiki logo"

# Add the needed LocalSettings
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php

# Install sqlite & check
./mw mwdd mediawiki install
curl -s -L http://default.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Turn on mysql, install & check
./mw mwdd mysql create
./mw mwdd mediawiki install --dbname mysqlwiki --dbtype mysql
curl -s -L http://mysqlwiki.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Destroy it all
./mw mwdd destroy
docker ps | wc -l | grep -q "1"
