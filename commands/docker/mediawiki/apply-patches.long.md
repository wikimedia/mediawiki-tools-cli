# MediaWiki apply-patches

Fetch Gerrit changes into your local MediaWiki development environment.

Given one or more Gerrit change numbers, this command will:

1. Query the Gerrit API to determine the project, ref, and fetch URL
2. Map the project to the correct local directory (core, extension, or skin)
3. Ensure the repository exists locally (or clone it with `--clone-missing`)
4. Stash any uncommitted local changes
5. Fetch the change ref
6. Apply it using the selected mode

By default, `Depends-On:` footers from commit messages are resolved and applied first.
Dependency cycles are detected and handled safely (no infinite recursion).

When a `Depends-On` value is a non-unique Change-Id, resolution uses the start branch
context (default `master`). Set `--start-branch` to match your target train/release branch
for deterministic dependency resolution.

If a dependency points to a repository that is not present locally, the command
fails by default. Use `--clone-missing` to allow automatic cloning.

Modes:

- `checkout` (default): check out `FETCH_HEAD` as a detached HEAD (good for quickly testing exact patchset state)
- `cherry-pick`: cherry-pick `FETCH_HEAD` into your current branch (good for stacking local changes)

This is useful for quickly testing patches from Gerrit in your local dev environment,
similar to [Patch Demo](https://www.mediawiki.org/wiki/Patch_demo) but running locally.

No Gerrit authentication is required.
