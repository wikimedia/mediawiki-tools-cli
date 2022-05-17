<?php

// Simple wrapper around install.php to enable us to set a umask for the process.
// This means any files created, such as sqlite dbs, will be accessible by all.

// Set a umask for MediaWiki as we are in a development environment
// This is also currently at the top of LocalSettings.php for regular execution
umask(000);

require_once( '/var/www/html/w/maintenance/install.php' );
