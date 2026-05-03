# MediaWiki apply-patches

Apply Gerrit changes to your local MediaWiki development environment.

Given one or more Gerrit change numbers, this command will:

1. Query the Gerrit API to determine the project, ref, and fetch URL
2. Map the project to the correct local directory (core, extension, or skin)
3. Clone the repository if it is not already present locally
4. Stash any uncommitted local changes
5. Fetch the change ref and cherry-pick it

This is useful for quickly testing patches from Gerrit in your local dev environment,
similar to [Patch Demo](https://www.mediawiki.org/wiki/Patch_demo) but running locally.

No Gerrit authentication is required.
