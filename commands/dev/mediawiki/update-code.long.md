# Updating development source checkouts

`mw dev mediawiki update-code` updates MediaWiki core and each checked-out skin
and extension in the selected development code directory. Core is processed
first; repositories are then handled independently, so a failure in one does
not undo successful updates in others. The directory comes from
`MEDIAWIKI_VOLUMES_CODE`; use `mw dev mediawiki get-code` to set it up.

The default `--mode=ff-only` fast-forwards clean checkouts on their current
tracking branches. Dirty worktrees (including untracked files), detached HEADs,
and branches without an upstream are skipped and reported. Diverged branches
and fetch failures are errors. Local commits are never rebased automatically.

`--mode=stash` temporarily saves tracked and untracked changes, advances the
current branch, then reapplies the changes with their staged and unstaged state
preserved. Before touching the checkout, it verifies the saved changes in a
temporary worktree. If the update or reapplication fails, it attempts to restore
the original branch, commit, index, and worktree, and reports a nonzero status.
Detached HEADs and branches without an upstream are skipped; diverged branches
are left unchanged.

`--mode=mop` saves tracked and untracked changes and leaves them saved. It
switches to `origin`'s advertised default branch (usually `main` or `master`) and
resets that branch to the remote tip; it can also start from a detached HEAD.
Before resetting, it preserves the previous HEAD and existing local default
branch tip under recovery refs, so local commits remain reachable. It will not
take over a default branch checked out in another worktree. A dry run infers the
default branch from local remote refs; an actual update asks `origin`.

Both stash modes retain saved changes even after success or automatic recovery.
Output includes stable refs under `refs/mwcli/update-code/` for the original
commit, prior default-branch tip when applicable, and saved changes. Ignored
files are not stashed, and an update that would overwrite them is refused. Finish
any merge, rebase, or cherry-pick before using these modes. Submodules are
handled individually; dirty nested worktrees that a superproject stash cannot
save prevent a reset. If automatic restoration fails, the command stops and
reports `recovery-needed` with the refs; it does not claim the original state
was restored. See `mw help guides dev update-code` for recovery commands and
examples.

Git-only updates and `--dry-run` do not require Docker. `--composer` runs
Composer update in the MediaWiki PHP container only after every source checkout
succeeds with none skipped. Docker is needed for this step; source updates are
not rolled back if Composer fails. The command does not run database maintenance;
run maintenance explicitly after reviewing source and dependency changes.
