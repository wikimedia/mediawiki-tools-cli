# Changelog

All notable changes to this project will be documented in this file.

Each tagged release MUST have a section 2 heading starting at the time of release `## TAG-NAME...` or Gitlab release notes will be missed.

## Unreleased

- Added URL to the `docker mediawiki sites` command output
- Improved error page when trying to access a non installed site to show the `mediawiki sites` command.
- Improved `update` changelog output, adding a newline between the different versions if many are upgraded through.
- Improved global command validation when using `--help`
- Development environment (`mw docker`)
  - Image updates:
    - postgres 13.18 -> 13.20
    - node20-test-browser 20.18.1 -> 20.19.1
    - shellbox 2025-01-12-210619-php-rpc81 -> 2025-04-16-162025-php-rpc81
- Improved README.

## v0.28.0

- Added `mw wiki page list` command
- Added `--dry-run` option to all `mw wiki page` commands
- Added ability to pass a list of title to `mw wiki page delete`
- Fixed issue with global command flags being parsed and used in the `docker` command.

## v0.27.2

- Revert to using the `buster-php81-fpm:1.0.1-s2` image by default for the `mediawiki` services as `bookworm-php83-fpm:1.0.0` contains a version of `libcurl` that breaks mwcli (T388411)

## v0.27.1

- Fixed bug if elasticsearch service created and `ELASTICSEARCH_PORT` env var is not set (T387729)

## v0.27.0

- Added the idea of a `--output web` type, which will open the output in a web browser.
  - Used in `mw version`
  - Used in `mw codesearch search`
- Added an option to `docker mediawiki get-code` to save the Gerrit username and interaction type to the config file.
- Improved `update` command output, including new real progress bar (related to T369835 & T368018)
- Improved configuration handling, including less disk reads and writes.
  - Configuration can now have defaults, and be overridden by environment variables with the `MWCLI_` prefix.
- Improved `gerrit` command config by merging with main config file, and command supports `--username` and `--password` flags.
- Improved many error cases, returning errors, rather than panics.
- Fixed exit codes in the case non-existent commands are run, where help text used to be displayed (T293062)
- Fixed command `completion` when no config file exists (T330310)
- Fixed panics when receiving 401 responses from `gerrit` API commands, now correctly errors.
- Development environment (`mw docker`)
  - Added exposure `mysql`, `redis`, `elasticsearch` and `postgres` services by default to ephemeral ports.
  - Added `*_PORT` environment variables to `mysql`, `redis`, `elasticsearch` and `postgres` services, allowing local mapping.
  - Fixed order of custom yml files (not positioned last) to enabled overriding of default services and values.
  - Fixed issue with `docker compose` commands and images on linux arm systems, by adding `DOCKER_DEFAULT_PLATFORM=linux/amd64` in those situations (T355341)
  - Fixed use of deprecated `${var}` format in `MwddSettings.php` in favour of `{$var}` format. Thanks @dcausse!
  - Image updates:
    - buster-php81-fpm -> bookworm-php83-fpm (Including composer 2.8.3)
    - buster-apache2 -> bookworm-apache2

You might also be interested in this blog post looking at the [usage of this tool](https://addshore.com/2025/01/mwcli-a-mediawiki-focused-command-line-tool-targeting-developers-over-the-years/).

## v0.26.1

- Improved and fixed `update` output when updating to versions where the changelog can't be shown

## v0.26.0

- Updated `gitlab` command to [1.52.0](https://gitlab.com/gitlab-org/cli/-/releases/v1.52.0)
- Development environment (`mw docker`):
  - Fixed podman issue, `mw docker mediawiki install` now creates a cache dir. By @hoo
  - Fixed parsing of --option=arg as env variable. By @migr
  - Fixed docs in `mw update --version` example. By @migr
  - Image updates:
    - quibble-buster-php83:1.8.0 -> 1.9.1
    - node20-test-browser:20.16.0 -> 20.18.1
    - shellbox images 2024-06-13-133425 -> 2025-01-12-210619...81

## v0.25.1

- Development environment (`mw docker`):
  - Image updates: repos/search-platform/cirrussearch-elasticsearch-image:v7.10.2-5 -> repos/search-platform/cirrussearch-elasticsearch-image:v7.10.2-5 (T369811)
  - Image updates: docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2024-06-10-140015 -> 2024-06-13-133425

## v0.25.0

- Development environment (`mw docker`):
  - Fixed extension git remotes in `get-code` command, which were being set incorrectly
  - Added `mw docker jaeger` service
  - Added PHP `memory_limit` setting to default mediawiki settings (2 GiB, mirroring WMF production)
  - Added a limit of 1 second to the main `destroy` command, so things get killed faster
  - Improved handling of git code fetches when no gerrit username is provided
  - Updated some documentation
  - Image updates:
    - postgres:13.14 -> 13.15
    - docker-registry.wikimedia.org/releng/quibble-buster-php83:1.7.0-s2 -> 1.8.0
    - docker-registry.wikimedia.org/wikimedia/eventgate-wikimedia:2023-10-25-155509-production -> 2024-06-11-192310-production
    - docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2024-05-04-233133-* -> 2024-06-10-140015-*
- Added `--dry-run` option to `mw update` command to show what would be updated without actually updating.

Thanks additionally to @hoo, @lucaswerkmeister-wmde, @audreypenven for patches this release.

## v0.24.3

- Fixed updating, which would ommit the `v` while fetching files for auto update (since v0.24.0).

If you ended up on `v0.24.0` and are unable to udpate, you can still update using `mw update --version v0.24.3`.

## v0.24.2

- Fixed `slice out of bounds` on retrieving changelog after a successful `mw update` command.

## v0.24.1

- Added default `HOME` value to `/` for `mw docker mediawiki composer` command. Required in some podman situations with `composer` and `podman`.
- Improved error message for `docker <service> expose` commands when container is not running
- Improved layout of grouped sub commands
- Minor internal package adjustments

## v0.24.0

- Added the ability to update to development builds if specifically requested.
- Development environment (`mw docker`):
  - Use `docker compose` by default, rather than `docker-compose` (T283484)
  - Disable SELinux labels for all containers with volumes.
  - `podman` should now work as a drop in replacement for `docker` (T291348)
  - Added `mw docker mediawiki npm` and `npx` commands as shortcuts
  - Added `mw docker mediawiki foreachwiki` command
  - Added `mw docker hosts show` command
  - Added `mw docker citoid` service
  - Added `LocalSettings.d` support for MediaWiki settings
  - Improved `get-code` to also fetch submodules (T353780)
  - Improved `get-code` to not be confusing if there is nothing to clone (T342080)
  - Remove `versions` from docker-compose files to avoid warnings
  - Default image updates: eventgate, node, shellbox, postgres, quibble, fresh

## v0.23.0

- Development environment (`mw docker`):
  - Added `wdqs` and `wdqs-ui` services (T292900)
  - Added a require of `PlatformSettings.php` within the development environment settings file
    - You no longer need to have a commented out `PlatformSettings.php` require line in your LocalSettings.php
  - Improved `get-code` command to exit early if there is nothing to do (T342393)
  - Fix automatic setting of `PORT` and `NETWORK_SUBNET_PREFIX` on `docker mediawiki *` commands (T338235)
  - Fix shellbox-media access from mediawiki (T333183)
  - Image updates:
    - mariadb from 10.9 to 10.11
    - postgres from 13.10 to 13.11
    - releng/node16-test-browser from 0.1.0 to 0.2.0
    - releng/quibble-buster-php81 from 1.5.3 to 1.5.5
    - dev/buster-php81-fpm from 1.0.0 to 1.0.1
    - dev/buster-apache2 from 2.0.0-s1 to 2.0.1
    - wikimedia/eventgate-wikimedia from 2023-02-14-162241-production to 2023-06-22-124213-production
    - wikimedia/mediawiki-libs-shellbox from 2023-03-12-165531 to 2023-05-01-213815

## v0.22.1

- Fixed `mw docker update` which was panicking (T332336)

## v0.22.0

- Added various boolean flags to `wiki page put` command (T331215)
- Added many `gerrit` command. See `mw gerrit --help` for more information.
  - Old gerrit commands based on ssh no longer exist, please use these new API based commands
  - `mw gerrit project current` is now `mw gerrit dotgitreview project`
- Removed spam about "choose a development environment mode" as there is only currently 1 mode.
- Development environment (`mw docker`):
  - Added a continuous job runner, see `mediawiki jobrunner`
  - Added ability to use multiple `custom` service sets (T327069)
  - Added `mediawiki mwscript` command, as a shortcut to the new `maintenance/run.php` script in MediaWiki (T332209)
  - Added `<service> image` commands for getting, setting and resetting an services image override (T330954 & T330955)
  - Added `env has` command to check if an environment variable is set
  - Updated `fresh` to use node 16 image. (T331993)
  - Fixed a verbose error in `mediawiki doctor` when a site was inaccessible
  - Image updates:
    - postgres from 13.9 to 13.10
    - docker-registry.wikimedia.org/releng/quibble-buster-php81 from 1.5.1 to 1.5.3
    - docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox image group to 2023-03-12-165531

## v0.21.0

- Fixed `--no-interaction` not working in some situations (T330307)
- Development environment (`mw docker`):
  - Added check to see if docker is running before commands execute (T329920)
  - Fixed slow DNS lookups when disconnected from the internet, which caused slow MediaWiki requests (T326735)
  - Added `mediawiki doctor` checks:
    - Check if `vendor` directory exists (T330926)
    - Check if a site has been installed (T330928)
    - Check if a site is accessible (T330929)
    - Check if container image overrides are set (T331136)
  - Image updates:
    - docker-registry.wikimedia.org/releng/quibble-buster-php81:1.4.7-s3 to 1.5.1
    - docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox images to 2023-02-24-002648

## v0.20.0

- Added XDG standards usage for config directory location (T305150)
- Added persistent Gerrit HTTP authentication for cli commands
- Added `--files`, `--repos` and `--exclude-files` flags to `codesearch search` command
- Added helpful information for the user when service are created, such as `mailhog`, `graphite`, `adminer` and `phpmyadmin`
- Added long documentation text for the `graphite`, `adminer` and `phpmyadmin` commands
- Updated `gitlab` command to [1.25.3](https://gitlab.com/gitlab-org/cli/-/releases/v1.25.3)
- Fixed `mw gerrit` command output for commands that used ssh
- Removed any suggestions to `sudo` using the CLI, instead providing alternative options
- Development environment (`mw docker`):
  - Added tab completion when `MEDIAWIKI_VOLUMES_CODE` is entered via wizard
  - Added `docker update` command to pull and update all created containers
  - Added `docker hosts where` command to show you where the hosts file is
  - Added `docker <service> expose` command for most services, exposing an internal port locally (T299514)
  - Added the ability to run multiple separate development environments via the `--context` flag (T301002)
  - Added the `--force-recreate` command to service `create` commands (T313411)
  - Added `docker mediawiki get-code` command for fetching MediaWiki, skins and extensions from Gerrit
  - Added notes about Windows hosts files when using WSL and the `docker hosts` commands
  - Added `docker mediawiki doctor` command to help find common issues
  - Removed code clone wizard from `docker mediawiki` command startup, instead prompting users to use `docker mediawiki get-code`
  - Image updates:
    - postgres `postgres:13.6` -> `postgres:13.9`

## v0.19.1

- Development environment (`mw docker`):
  - Fixed `MW_DB` handling for `mediawiki exec` commands 

## v0.19.0

- Added `config get` and `config set` commands
- Updated `gitlab` command to [1.23.0](https://gitlab.com/gitlab-org/cli/-/releases/v1.23.0)
- Development environment (`mw docker`):
  - Image updates:
    - mysql `mariadb:10.8` -> `mariadb:10.9.4`
    - mediawiki `d-r.wm.o/dev/buster-php74` -> `d-r.wm.o/dev/buster-php81`
    - quibble `d-r.wm.o/dev/quibble-buster-php74` -> `d-r.wm.o/dev/quibble-buster-php81`

## v0.18.0

- Added more command examples
- Improved top level short command descriptions
- Development environment (`mw docker`):
  - Added long form `docker-compose` command description
  - Fix incorrect grouping of `keycloak` command
  - Image updates:
    - `eventgate-wikimedia`: `2022-06-07-105344-production` -> `2022-11-28-190331-production`
    - `graphite-statsd`: `1.1.10-3` -> `1.1.10-4`
    - `releng/quibble-buster-php74`: `1.4.4` -> `1.4.7-s1`
    - `dev/buster-php74-fpm`: `1.0.0-s1` -> `1.0.0-s3`
    - shellbox* `2022-03-10-142520` -> `2022-12-05-111819`

## v0.17.0

- Added grouping of commands in help command output
- Added support for advanced outputs in `version` command
- Added `output` help command explaining how to use `--output`, `--filter` and `--format`
- Added integer support to `--filter`
- Added support for `*string` and `string*` string filters with `--filter`
- Improved `ziki` location selection, command description and some invalid input handling
- Fix document indentation during markdown rendering

- Development environment (`mw docker`):
  - Add `mediawiki sites` commands to list recently installed sites
  - Add ability to pass an `--ip` to `hosts` commands
  - Improved speed of `restart` commands by internally using `docker-compose restart` (T314894)

## v0.16.0

- Development environment:
  - Fix `docker exec` commands always have a 0 exit code (T307583)
  - Image updates:
    - `eventgate-wikimedia`: `2022-05-10-150602-production` -> `2022-06-07-105344-production`
    - `graphite-statsd`: `1.1.10-1` -> `1.1.10-3`

## v0.15.0

- Fix `no binary release found` when running `mw update` on Windows (T309450)
- Update mediawiki image to `buster-php74-fpm:1.0.0-s1` which includes `composer` version `2.1.8` (T311821)

## v0.14.0

- Add `config where` command to show where the config is located
- Add `tools exec` and `tools cp` commands for interacting with WMF Tools (T308928)
- Fix verbose output when updating (T302216)
- Fix `toolhub` commands error (T308929)
- Fix various typos in inline documentation
- Development environment:
  - Add a `keycloak` service (T309053)
  - Fix settings of wgStatsdServer for graphite service (T307365)
  - Image updates
    - `graphiteapp/graphite-statsd:1.1.8-8` -> `graphiteapp/graphite-statsd:1.1.10-1`
    - `eventgate-wikimedia:2022-02-01-141357-production` -> `eventgate-wikimedia:2022-05-10-150602-production`

Thanks to @ollieshotton, @cicalese, @samtar, @addshore for patches this release.

## v0.13.1

- Fix files being created as owned by root when using `sudo` as part of the suggested update path (T306981)
  - These files will now be created as the user running root where possible
  - The `~/.mwcli/.events` file will also not be recreated repeatedly, to avoid ownership changes

## v0.13.0

- Added `restart: unless-stopped` for most containers so that previously running containers are auto started after reboot (T305839)
- Added the ability to override images used for all services using environment variables (T306023)
- Added the ability to run multiple fresh (and quibble) commands simultaneously (T305683)
- Added the `restart` command to `stop` and then `start` the current running containers (T305943)
- Added `mysql mysql` and `mysql-replica mysql` commands that run the `mysql` cli in the mysql containers (T306864)
- Changed `resume` to `start` with a backward compatible alias (T305823)
- Changed `suspend` to `stop` with a backward compatible alias (T305823)
- Improved HTML error message when MediaWiki database can not be found, including commands that might help (T305099)
- Fix duplicated sub commands of `mw docker custom`
- Fix duplicated and broken sub commands of `mw docker shellbox`

## v0.12.1

- Fixed `glab` commands that make use of a `-v` flag
- Updated various docker dev environment images:
  - mediawiki-web `buster-apache2:1.0.0-s1` -> `buster-apache2:2.0.0`

## v0.12.0

- Added `cs` alias for `codesearch` command
- Added `gl` alias for `gitlab` command
- Improved syncing of file permissions for dev environment files
- Updated various docker dev environment images:
  - eventgate `2021-10-21-192154-production` -> `2022-02-01-141357-production`
  - graphite `1.1.8-2` -> `1.1.8-8`
  - quibble `quibble-buster-php74:1.3.0-s1` -> `quibble-buster-php74:1.4.4`
  - mediawiki `stretch-php73-fpm:3.0.0` -> `stretch-php74-fpm:3.0.0`
  - mediawiki-web `stretch-apache2:2.0.0` -> `buster-apache2:1.0.0-s1`
  - mariadb `10.7` -> `10.8`
  - postgres `13.5` -> `13.6`
  - shellbox* `2022-01-06-073153` -> `2022-03-10-142520`

## v0.11.0

- Added `gerrit ssh` command
- Added `gerrit api `command
- Added `--project` option to `gerrit changes` command
- Improved verbose output flag to enable use of `-v` and `-vv` (T301691)
- Improved output formats of some commands, including `--output`, `--format`, `--filter` including new `json` output
  - `gerrit changes`
  - `codesearch`
- Improved output when binaries are needed on disk (such as docker) that do not exist (T301557)
- Fix display of help when search term is missed in `codesearch search` command
- Fix display of help when search term is missing in `toolhub tools search` command
- Fix indenting of help and usage text across commands
- Fix `codesearch search` commands usage when spaces appear in the search text (needed urlencoding) (T301973)
- Fix color usage in output when not in a TTY across commands
- Fix mistaken INFO log of periodical version check output
- Fix tab completion for `docker` command (T301693)
- Updated docker dev environment fresh image to `node14-test-browser:0.0.2-s4`
- Added easter eggs ;)

Thanks to @bpirkle & @ollieshotton for patch submissions
Thanks to @itamar, @ollieshotton for bug reports & requests

## v0.10.2

- Fixed handling for relative paths not starting with `./` during initial MediaWiki setup wizard (T300867)
- Fixed handling for windows absolute paths that look like `D:\` etc during initial MediaWiki setup wizard (T300867)
- Logging now uses the `logrus` library, so verbose output has changed slightly (T301005)

## v0.10.1

- Fixed wizards prompting on `destroy` commands (T292331)
- Fixed telemetry question being asked again if being run with sudo (T300412)
- Fixed telemetry on `docker env` commands

## v0.10.0

- Added a progress bar while the `update` command is downloading an update (T293586)
- Improved formatting of release notes once `update` command has completed
- Updated `gitlab` command to [1.22.0](https://github.com/profclems/glab/releases/tag/v1.22.0)

Docker development environment:

- Added `where` command for the working directory of the development environment
- Added `mediawiki where` command for the MediaWiki directory
- Added `custom where` command for the `custom.yml` location
- Added 5 `shellbox <type>` service commands for commonly used shellbox services
- Improved `docker resume` output to not show "failed" services, that have never even been started (T299631)
- Improved formatting of long command descriptions
- Updated `nginx-proxy` image from `jwilder/nginx-proxy:0.9` to `jwilder/nginx-proxy:0.10` 
- Updated `mediawiki-fresh` image from `wm.o/releng/node14-test-browser:0.0.2` to `wm.o/releng/node14-test-browser:0.0.2-s3`
- Updated `mediawiki-quibble` image from `wm.o/releng//quibble-buster-php74:1.1.1` to `wm.o/releng//quibble-buster-php74:1.3.0-s1`
- Updated `mysql` images from `mariadb:10.6` to `mariadb:10.7`
- Updated `postgres` images from `postgres:13.2` to `postgres:13.5`
- Fixed handling for relative paths starting in `./` during initial MediaWiki setup wizard (T294177)
- Fixed Windows issue to do with file embedding `Failed to open file: embed\files.txt` (T295473)
- Fixed issue where MediaWiki would create an unreadable `mw-GlobalIdGenerator-UID-88` file and error (T293682)

## v0.9.0

- Added `wiki page push` command for updating a single MediaWiki page.
- Added help text when `exec` commands are run without arguments (T294851).
- Added optional telemetry submission via Wikimedia Event intake (T293583).
- Improved error message when `exec` commands are run without running containers.
- Improved on wiki documentation with auto generated command reference https://www.mediawiki.org/wiki/Cli/ref.

## v0.8.1

- Fixed development environment `exec` and internal command running with `docker-compose` version 2

## v0.8.0

Development environment specific:

- Added `custom` service set, usable by creating a `custom.yml` (see the help output for details)
- Updated `eventlogging`, `graphite`, `mediawiki-web` & `mariadb` image versions

## v0.7.0

- Added `codesearch` command
- Added the ability to `update` to a specific `--version` (including rollback)
- Fixed "dirty" state in verbose version output

Development environment specific:

- Fixed the chown of some directories on `mediawiki install`
- Fixed running `fresh` or `quibble` after a previous failed command
- Fixed default `fresh` and `quibble` environment variables
- Fixed typos in setup wizard

## v0.6.0

- Added `toolhub search` command.
- Added `--type` filter to `toolhub list` command.

Development environment specific:

- Added `eventlogging` service.
- Fixed removal of nonexistent volumes through some commands.
- Fixed regression in 0.5.0 with passing env vars into exec commands such as `mw docker mediawiki exec -- XDEBUG_SESSION=1 php test.php`

## v0.5.0

- Added `toolhub` command for `list`ing and `get`ing tools.
- Added `gerrit change list` command.
- Added `gerrit group members` command.
- Improved all prompt questions.
- Now also built targeting `darwin/arm64`.

Development environment specific:

- Added `elasticsearch` service.
- Added a `mailhog` service https://github.com/mailhog/MailHog.
- Fixed issues cloning MediaWiki and Vector with a non shallow clone during setup.
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
