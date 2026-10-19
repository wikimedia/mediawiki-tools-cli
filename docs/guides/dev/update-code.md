# Update a development wiki

This workflow updates a configured MediaWiki checkout, reviews the source
changes, then updates dependencies and database schemas. First [set up a wiki](setup.md)
and select the [development context](contexts.md) you want to update. Review or
back up any development data you need before running schema changes.

For update modes, skipped repositories, saved changes, and failure behavior,
read `mw dev mediawiki update-code --help`.

## Preview and update the source

Preview which checkouts are eligible without fetching or changing them:

```sh
mw dev mediawiki update-code --dry-run
```

Check the reported branches and skipped repositories. Then fast-forward eligible
clean checkouts:

```sh
mw dev mediawiki update-code
```

For uncommitted work you want reapplied after updating, use this instead:

```sh
mw dev mediawiki update-code --mode=stash
```

Review the results before continuing. In the core checkout and any extensions
or skins you changed, inspect the working tree and recent commits:

```sh
git status --short
git log -5 --oneline
git diff
git diff --cached
```

Source updates are independent: an error does not undo repositories that already
updated. Resolve failures and review skipped repositories before updating
shared dependencies or running maintenance.

## Update dependencies and schemas

With the environment running, update PHP dependencies after reviewing the
source changes:

```sh
mw dev mediawiki composer update --no-interaction
```

Alternatively, `mw dev mediawiki update-code --composer` combines source and
Composer updates, running Composer only when all checkouts succeed with none
skipped. Review its output before proceeding to database maintenance.

Update the configured development wikis:

```sh
mw dev mediawiki foreachwiki update.php -- --quick
```

Check the maintenance output, then open your wiki and verify the features you
are developing. Composer and database changes are separate from Git recovery;
restoring source refs does not reverse either of them.

## Recover saved work after a reset

If you deliberately want to switch to the remote default branch and leave local
changes saved, preview mop mode before using it:

```sh
mw dev mediawiki update-code --mode=mop --dry-run
mw dev mediawiki update-code --mode=mop
```

In each affected repository, use the exact refs printed by the command. The
following placeholders stand for those printed refs. Inspect the saved work
before applying it:

```sh
git show --stat <printed-original-ref>
git stash show --stat <printed-stash-ref>
```

To preserve the old commit on a named branch:

```sh
git branch recovered-work <printed-original-ref>
```

To reapply saved changes to your current checkout, start from a clean worktree
and use:

```sh
git stash apply --index <printed-stash-ref>
```

Resolve any conflicts and inspect `git status` and `git diff` before continuing.
If the CLI reports `recovery-needed`, inspect the repository and its printed
recovery refs first; do not assume it restored the original state.
