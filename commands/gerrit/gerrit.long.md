# Wikimedia Gerrit

Save HTTP credentials with `mw gerrit auth login --username YOURNAME`.
Use `auth profiles` to list accounts, `auth default --profile NAME` to choose a
default, and `--profile NAME` on any Gerrit command to select another account.
`auth remove --profile NAME` removes local credentials without revoking them.

SSH commands use the selected profile's username and your configured SSH keys.
Your SSH agent/config must support connecting to `gerrit.wikimedia.org`.

## Continuous integration

`mw gerrit ci status 1372985` shows public Jenkins results reported in Gerrit
alongside live Zuul jobs. Pass `--watch` to see changes as they happen. Use a
Gerrit change URL or number, or omit it to inspect the uploaded patchset that
matches your checkout's current commit. A patchset in the URL or in
`--patchset` selects that revision explicitly.

`mw gerrit ci logs 1372985 --patchset 2 --job mediawiki-core-phan-only-php83`
prints the Jenkins console log. When exactly one failed job has a log, you
can omit `--job`. Use `--pipeline` if the same job runs in multiple pipelines.
Public CI reads work without a Gerrit account.

## Documentation

- [Gerrit SSH Access](https://www.mediawiki.org/wiki/Gerrit/Tutorial#Set_Up_SSH_Keys_in_Gerrit)
