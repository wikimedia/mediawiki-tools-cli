# MediaWiki apply-patches

Fetch Gerrit changes into your local MediaWiki development environment.

Given one or more Gerrit change numbers, this command will:

1. Query the Gerrit API to determine the project, ref, and fetch URL
2. Map the project to the correct local directory (core, extension, or skin)
3. Clone the repository if it is not already present locally
4. Stash any uncommitted local changes
5. Fetch the change ref
6. Apply it using the selected mode

Modes:

- `checkout` (default): check out `FETCH_HEAD` as a detached HEAD (good for quickly testing exact patchset state)
- `cherry-pick`: cherry-pick `FETCH_HEAD` into your current branch (good for stacking local changes)

This is useful for quickly testing patches from Gerrit in your local dev environment,
similar to [Patch Demo](https://www.mediawiki.org/wiki/Patch_demo) but running locally.

No Gerrit authentication is required.
