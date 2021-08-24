# Changelog

## 1.0.0 (Work in progress)

* Enable updates from releases.wikimedia.org

## [https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210806.1 v0.1.0-dev-addshore.20210806.1]

* Fix mysql server db check complaining about Countable ([phabricator](https://phabricator.wikimedia.org/T287695))
* Prepare for releases from releases.wikimedia.org
* Take backups of LocalSettings incase they get lost
* Create a user .composer directory if it doesn't exist ([phabricator](https://phabricator.wikimedia.org/T288309))

## [https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210714.1 v0.1.0-dev-addshore.20210714.1]

* Replace docker command with mwdd functionality
* Introduce a dev alias for use with your main development environment command
* Introduced basic cli configuration and config command

## [https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210703.1 v0.1.0-dev-addshore.20210703.1]

* Improve updater output
* mwdd
** Removed the confusing mwdd create command
** Implemented mwdd suspend and mwdd resume
** Fix most --user options for most exec commands
** Remove duplicate phpunit command

## [https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210627.1 v0.1.0-dev-addshore.20210627.1]
[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210524.1...v0.1.0-dev-addshore.20210627.1 Commits]

* mwdd: Use docker-compose 3.7 file versions
* mwdd: Use stretch-php72-fpm:3.0.0 image for MediaWiki, which fixed XDebug issues

## v0.1.0-dev-addshore.20210524.1
[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210523.2...v0.1.0-dev-addshore.20210524.1 Commits]

* Allow users to choose if they update or not
* Check for new updates daily
* mwdd: Make use of a composer cache
* mwdd: Fix permissions of data and log mounts
* mwdd: Internally use maintenance/checkComposerLockUpToDate.php
* mwdd: Add exec commands for all services 

## v0.1.0-dev-addshore.20210523.2

[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210523.1...v0.1.0-dev-addshore.20210523.2 Commits]

Initial addshore dev build of most mwdd functionality.