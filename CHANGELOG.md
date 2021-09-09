# Changelog

## 1.0.0 (Work in progress)

...


## [v0.1.0-dev-addshore.20210909.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210909.1)

* `mw update`: Stop printing update success release notes twice
* `mw dev docker-compose` no longet breaks if passed no arguments
* `mw dev mediawiki`: Switch default MediaWiki PHP version to 7.3
* `mw dev mediawiki`: Include `php-ast` in MediaWiki container
* `mw dev mediawiki`: Output details of username, password and domain of MediaWiki site after install
* `mw dev mediawiki`: Nicer error from MediaWiki if no DB exists when loading a site
* `mw dev mediawiki install`: now requires that you specify a `--dbtype`
* DEV: `make`: Fix generation of staticfiles using make

## [v0.1.0-dev-addshore.20210907.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210907.1)

* Enable updates from releases.wikimedia.org
* Fix segfaults caused by xdebug and `xdebug.var_display_max_` -1 values. ([phabricator](https://phabricator.wikimedia.org/T288363))
  * MediaWiki no longer has `ini_set( 'xdebug.var_display_max_depth', -1 );` set
  * MediaWiki no longer has `ini_set( 'xdebug.var_display_max_children', -1 );` set
  * MediaWiki no longer has `ini_set( 'xdebug.var_display_max_data', -1 );` set

## [v0.1.0-dev-addshore.20210806.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210806.1)

* Fix mysql server db check complaining about Countable ([phabricator](https://phabricator.wikimedia.org/T287695))
* Prepare for releases from releases.wikimedia.org
* Take backups of LocalSettings incase they get lost
* Create a user .composer directory if it doesn't exist ([phabricator](https://phabricator.wikimedia.org/T288309))

## [v0.1.0-dev-addshore.20210714.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210714.1)

* Replace docker command with mwdd functionality
* Introduce a dev alias for use with your main development environment command
* Introduced basic cli configuration and config command

## [v0.1.0-dev-addshore.20210703.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210703.1)

* Improve updater output
* mwdd
** Removed the confusing mwdd create command
** Implemented mwdd suspend and mwdd resume
** Fix most --user options for most exec commands
** Remove duplicate phpunit command

## [v0.1.0-dev-addshore.20210627.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210627.1)
[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210524.1...v0.1.0-dev-addshore.20210627.1 Commits]

* mwdd: Use docker-compose 3.7 file versions
* mwdd: Use stretch-php72-fpm:3.0.0 image for MediaWiki, which fixed XDebug issues

## [v0.1.0-dev-addshore.20210524.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210524.1)
[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210523.2...v0.1.0-dev-addshore.20210524.1 Commits]

* Allow users to choose if they update or not
* Check for new updates daily
* mwdd: Make use of a composer cache
* mwdd: Fix permissions of data and log mounts
* mwdd: Internally use maintenance/checkComposerLockUpToDate.php
* mwdd: Add exec commands for all services 

## [v0.1.0-dev-addshore.20210523.2](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210523.2)

[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210523.1...v0.1.0-dev-addshore.20210523.2 Commits]

Initial addshore dev build of most mwdd functionality.
