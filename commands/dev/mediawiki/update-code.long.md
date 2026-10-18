# Updating development source checkouts

Update core and checked-out skins/extensions. Preview any mode without contacting
remotes or changing source checkouts:

```sh
mw dev mediawiki update-code --dry-run
mw dev mediawiki update-code
mw dev mediawiki update-code --mode=stash
mw dev mediawiki update-code --mode=mop
```

Git-only updates and dry runs do not require Docker to be installed or running.
The code directory is read from `MEDIAWIKI_VOLUMES_CODE` in the selected development
context. Configure it first with `mw dev mediawiki get-code`.

## Modes

The default, `--mode=ff-only`, fast-forwards clean checkouts on their existing
tracking branches. Dirty worktrees (including untracked files), detached HEADs
and branches without an upstream are skipped and listed. Diverged branches or
failed fetches return an error while independent repositories are still checked.

`--mode=stash` saves tracked and untracked changes, fast-forwards the current
branch, and reapplies the changes with their staged/unstaged state preserved.
It checks the saved changes in a temporary worktree before advancing the original
checkout. If the update or reapplication fails, it restores the original branch,
commit, index and worktree and reports `restored` with a nonzero exit status.
Detached HEADs and branches without an upstream are still skipped. Local commits
are not rebased; a diverged branch is left unchanged and reported as an error.

`--mode=mop` saves tracked and untracked changes and leaves them saved. It fetches
`origin`, switches to its advertised default branch (usually `main` or `master`),
and resets that local branch to the remote tip. It can also start from a detached
HEAD. The previous HEAD and any existing local default-branch tip are preserved
under recovery refs before reset, so local commits remain reachable. A default
branch checked out in another worktree is not forcibly taken over. Dry-run uses
local remote refs to infer the default branch; the real update asks `origin`.

Both stash modes retain the saved stash even after a successful update or
restoration. Output identifies stable recovery refs under
`refs/mwcli/update-code/`, including `/original`, `/default` when applicable, and
`/stash` when there were changes. For example, use `git stash apply --index` with
the printed stash ref to restore saved changes, or create a recovery branch from
the printed original commit ref. Existing stash entries are not popped or dropped.
Ignored files are not stashed; an update that would overwrite them is refused.
Finish any merge, rebase or cherry-pick before using these modes.

Core is processed first. Submodules are handled individually, without recursive
Git checkout behavior. A superproject stash cannot save dirty nested worktrees;
if it remains dirty after stashing, it is restored and reported instead of reset.
Each repository is independent: successful updates are retained if another fails.
If automatic restoration itself fails, the batch stops and reports `recovery-needed`
alongside the recovery refs; it does not claim the original state was restored.

## Composer and database updates

`--composer` runs Composer update in the MediaWiki PHP container only after every
source checkout succeeds, with none skipped. Docker is required for this step;
`--dry-run --composer` only previews it. Source updates are not rolled back if
Composer fails. Without this flag, Composer is not run.

This command does not run database maintenance. After reviewing source/dependency
changes, use maintenance explicitly, for example:

```sh
mw dev mediawiki foreachwiki update.php -- --quick
```
