#!/usr/bin/env bash

# Setup & Create
./mw mwdd env set MEDIAWIKI_VOLUMES_CODE $(pwd)/mediawiki
./mw mwdd create
./mw mwdd mysql create
echo "<?php" >> mediawiki/LocalSettings.php
echo "//require_once "$IP/includes/PlatformSettings.php";" >> mediawiki/LocalSettings.php
echo "require_once '/mwdd/MwddSettings.php';" >> mediawiki/LocalSettings.php

# Validate the basic stuff
./mw mwdd docker-compose ps
./mw mwdd env list
cat ~/.mwcli/mwdd/default/.env

# Install & check
./mw mwdd mediawiki install --dbname mysqlwiki --dbtype mysql
curl -s -L http://mysqlwiki.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Suspend and resume and check the site is still there
./mw mwdd mysql suspend
./mw mwdd mysql resume
curl -s -L http://mysqlwiki.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Destroy and restart mysql, reinstalling mediawiki
./mw mwdd mysql destroy
./mw mwdd mysql create
./mw mwdd mediawiki install --dbname mysqlwiki --dbtype mysql
curl -s -L http://mysqlwiki.mediawiki.mwdd.localhost:8080 | grep -q "MediaWiki has been installed"

# Destroy it all
./mw mwdd destroy
docker ps | wc -l | grep -q "1"
