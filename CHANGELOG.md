# Changelog

All notable changes to this project will be documented in this file.

Each tagged release MUST have a section 2 heading starting at the time of release `## TAG-NAME...` or Gitlab release notes will be missed.

## Unreleased

- ...

## v0.5.0

- Added `toolhub` command for `list`ing and `get`ing tools.
- Added `gerrit change list` command.
- Added `gerrit group members` command.
- Improved all prompt questions.
- Now also built targetting `darwin/arm64`.

Development environment specific:

- Added `elasticsearch` service.
- Added a `mailhog` service https://github.com/mailhog/MailHog.
- Fixed issues cloning MediaWiki and Vecotr with a non shallow clone during setup.
- Fixed SQLite permission issues.
- Fixed issue with using `maintenance/shell.php`.
- Fixed some `quibble` commands.
- Fixed trying to save `/etc/hosts` file even when nothing had changed.

This release was made on 17th October 2021.

## v0.4.0

- Added `gerrit` command with `project` subcommand.
- Added `docker fresh` command.
- Added `docker memcached` command and service.
- Added `docker env clear` command to clear all environment variables.
- Added work in progress `docker quibble` command.
- Improved help output for the `docker redis` command.
- Fixed exit codes for various `docker hosts` commands.
- Fixed aborting of initial setup prompts for `docker mediawiki` (thanks Lens0021).
- Fixed typos throughout (thanks Lens0021).
- `$wgTmpDirectory` is no longer set by `docker mediawiki`, allowing the MediaWiki default to prevail.

This release was made on 15th October 2021.

## v0.3.0

- Added `gitlab` command for interacting with the Wikimedia Gitlab instance.
- Added `--no-interaction` option to all commands with user prompts.
- Changed update check period from 1 day to 3 hours.
- Fixed long wait when checking for update with no internet.
- Fixed fatals on regular update check failures.

This release was made on 4th October 2021.

## v0.2.1

- `mw docker mediawiki install`
  - Added long help message, explaining what the command does.
  - Fixed composer lockfile check & prompt for composer update.
  - Fixed moving and restoration of LocalSetting.php during install.
  - Fixed leaving .bak LocalSettings files around if we correctly move the file back.
- `mw docker mediawiki exec`
  - Added mediawiki log tail example.

## v0.2.0

This is the second release built by CI on Gitlab, but the first that will be served to users.
From this point forward users will automatically update from Gitlab releases.

- Added verbose flags to the `version` and `update` commands.
- Changed default output of the `version` command.
- Changed default output of the `update` command when no update is available, making the output more useful.
- Removed `update_channel` from the configuration, the only update channel is now Gitlab.


## v0.1.0-dev.20210920.1

There are no functionality changes in this release compared to `v0.1.0-dev-addshore.20210916.1`.

This is the first release built by CI on Gitlab.

## v0.1.0-dev-addshore: [addshore/mwcli development on github](https://github.com/addshore/mwcli)

### [v0.1.0-dev-addshore.20210916.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210916.1)

* `mw dev hosts`: command added to interact with your `/etc/hosts` file if needed
* `mw dev * exec`: commands can now have environment variables passed to them. e.g. `mw dev mediawiki exec -- FOO=bar env`
* `mw dev`: ports are now checked for availability before listening begins
* `mw dev adminer`: Updated from `adminer:4.8.0` to `adminer:4` (enabling minor update)
* Fix typos

### [v0.1.0-dev-addshore.20210910.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210910.1)

* `mw dev mediawiki phpunit`: Command has been removed, please use `mw dev mediawiki exec`
* `mw dev`: Use correct terminal size in all `exec` commands
* `mw dev destroy`: Fix command description

### [v0.1.0-dev-addshore.20210909.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210909.1)

* `mw update`: Stop printing update success release notes twice
* `mw dev docker-compose` no longer breaks if passed no arguments
* `mw dev mediawiki`: Switch default MediaWiki PHP version to 7.3
* `mw dev mediawiki`: Include `php-ast` in MediaWiki container
* `mw dev mediawiki`: Output details of username, password and domain of MediaWiki site after install
* `mw dev mediawiki`: Nicer error from MediaWiki if no DB exists when loading a site
* `mw dev mediawiki install`: now requires that you specify a `--dbtype`
* DEV: `make`: Fix generation of staticfiles using make

### [v0.1.0-dev-addshore.20210907.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210907.1)

* Enable updates from releases.wikimedia.org
* Fix segfaults caused by xdebug and `xdebug.var_display_max_` -1 values. ([phabricator](https://phabricator.wikimedia.org/T288363))
  * MediaWiki no longer has `ini_set( 'xdebug.var_display_max_depth', -1 );` set
  * MediaWiki no longer has `ini_set( 'xdebug.var_display_max_children', -1 );` set
  * MediaWiki no longer has `ini_set( 'xdebug.var_display_max_data', -1 );` set

### [v0.1.0-dev-addshore.20210806.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210806.1)

* Fix mysql server db check complaining about Countable ([phabricator](https://phabricator.wikimedia.org/T287695))
* Prepare for releases from releases.wikimedia.org
* Take backups of LocalSettings incase they get lost
* Create a user .composer directory if it doesn't exist ([phabricator](https://phabricator.wikimedia.org/T288309))

### [v0.1.0-dev-addshore.20210714.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210714.1)

* Replace docker command with mwdd functionality
* Introduce a dev alias for use with your main development environment command
* Introduced basic cli configuration and config command

### [v0.1.0-dev-addshore.20210703.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210703.1)

* Improve updater output
* mwdd
** Removed the confusing mwdd create command
** Implemented mwdd suspend and mwdd resume
** Fix most --user options for most exec commands
** Remove duplicate phpunit command

### [v0.1.0-dev-addshore.20210627.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210627.1)
[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210524.1...v0.1.0-dev-addshore.20210627.1 Commits]

* mwdd: Use docker-compose 3.7 file versions
* mwdd: Use stretch-php72-fpm:3.0.0 image for MediaWiki, which fixed XDebug issues

### [v0.1.0-dev-addshore.20210524.1](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210524.1)
[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210523.2...v0.1.0-dev-addshore.20210524.1 Commits]

* Allow users to choose if they update or not
* Check for new updates daily
* mwdd: Make use of a composer cache
* mwdd: Fix permissions of data and log mounts
* mwdd: Internally use maintenance/checkComposerLockUpToDate.php
* mwdd: Add exec commands for all services

### [v0.1.0-dev-addshore.20210523.2](https://github.com/addshore/mwcli/releases/tag/v0.1.0-dev-addshore.20210523.2)

[https://github.com/addshore/mwcli/compare/v0.1.0-dev-addshore.20210523.1...v0.1.0-dev-addshore.20210523.2 Commits]

Initial addshore dev build of most mwdd functionality.
