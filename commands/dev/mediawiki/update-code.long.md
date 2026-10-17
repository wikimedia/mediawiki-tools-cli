# Updating development source checkouts

Preview core and checked-out skins/extensions without contacting remotes:

```sh
mw dev mediawiki update-code --dry-run
```

Update eligible repositories from their existing tracking branches:

```sh
mw dev mediawiki update-code
mw dev mediawiki update-code --composer
```

Each pull is fast-forward only, with no rebase, reset or automatic branch switch.
Dirty worktrees (including untracked files), detached HEADs and branches without
an upstream are skipped and listed. Diverged branches or failed fetches return
an error while allowing independent repositories to be checked. Core is processed
first; submodules are updated individually on their own checked-out branches,
without recursive Git checkout behavior.

`--composer` runs Composer update in the MediaWiki PHP container only after every
source checkout is eligible and all Git operations succeed. It preserves the
container's platform checks and stops on failure. Source changes already made
are not rolled back if a later repository or Composer fails. Review the reported
results before retrying. Without this flag, Composer is not run.

This command does not repair databases or run maintenance scripts. After reviewing
source/dependency changes, use the existing maintenance commands deliberately,
for example `mw dev mediawiki foreachwiki update.php -- --quick`. Keep backups
before schema updates when development data matters.
