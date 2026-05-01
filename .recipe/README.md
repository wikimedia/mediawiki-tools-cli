# mwcli Dev Recipe YAML (draft)

This directory contains a draft recipe schema and example recipes derived from the copied MediaWiki-Docker extension setup pages.

Use with:

- `mw dev recipe --file /path/to/recipe.yaml`
- `mw dev recipe validate --file /path/to/recipe.yaml`

## Schema (v0.1)

```yaml
type: mwcli.dev/recipe            # required, also accepts mwcli/recipe
version: 0.1                      # required
name: my-recipe
description: Optional human-readable text

source:
  useGithub: false                # optional
  shallow: true                   # optional
  gerritInteractionType: ssh      # optional, defaults from mw config
  gerritUsername: addshore        # optional, defaults from mw config

env:
  SOME_ENV: value                 # optional, written to mwdd .env

customCompose:                    # optional, writes ${mwddDir}/<name>.yml
  name: custom
  content: |
    services:
      some-service: {...}

services:
  - name: mediawiki
    state: started                # started (default) or stopped
  - name: mysql

code:
  core: true
  extensions:
    - name: ContentTranslation
    - name: VisualEditor
  skins:
    - name: Vector

sites:
  - dbname: en
    dbtype: mysql                 # mysql, postgres, sqlite

jobRunner:                        # optional, manages mediawiki/jobrunner-sites
  sites:
    - en

localSettings:
  appendPHP: |
    wfLoadExtension( 'Foo' );
  yamlSettingsFile: settings/dev.yaml
  yamlSettings: |
    extensions:
      - Foo

maintenance:
  - name: update
    command: ["php", "maintenance/run.php", "update"]
    user: nobody                  # optional
    workingDir: /var/www/html/w   # optional
    env:                          # optional
      PHPUNIT_WIKI: en

# Stretch goal support (implemented as generic git fetch + cherry-pick)
patches:
  - name: some-gerrit-change
    repoPath: extensions/Foo      # path relative to MEDIAWIKI_VOLUMES_CODE or absolute
    fetch: ["https://gerrit.wikimedia.org/r/mediawiki/extensions/Foo", "refs/changes/34/1234/5"]
    cherryPick: FETCH_HEAD
```

## Notes

- Recipes are intentionally **environment-agnostic** YAML and can be stored in extension repos.
- `localSettings.yamlSettingsFile` + `localSettings.yamlSettings` are included to align with MediaWiki's YAML settings format recommendation.
- Extension `composer.json` merge includes are managed via `composer.local.json` and applied with a single `composer update --with-all-dependencies` run.
- `jobRunner.sites` is preferred over explicit `runJobs.php` recipe steps.
- `patches` is intentionally generic so it can later map to higher-level Patch Demo/Gerrit/GitHub PR abstractions.
