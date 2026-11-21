# Wikimedia Gerrit

Save HTTP credentials with `mw gerrit auth login --username YOURNAME`.
Use `auth profiles` to list accounts, `auth default --profile NAME` to choose a
default, and `--profile NAME` on any Gerrit command to select another account.
`auth remove --profile NAME` removes local credentials without revoking them.

SSH commands use the selected profile's username and your configured SSH keys.
Your SSH agent/config must support connecting to `gerrit.wikimedia.org`.

## Check your setup

Run `mw gerrit setup --check` to check Gerrit HTTP credentials and SSH
authentication separately. Inside a Git checkout, it also shows the effective
Gerrit fetch and push URLs, Git author email, and active `commit-msg` hook.
The check does not upload anything or change Git or SSH configuration.

Use `mw gerrit ssh-key add ~/.ssh/id_ed25519.pub` to register an existing public
key with the selected profile. This uses the profile’s Gerrit HTTP credentials.

Install the standard hook in an existing checkout with
`mw gerrit setup --install-hook`. Existing hooks and custom hooks directories
are preserved. Use `mw gerrit setup --push-protocol https` to change the push
URL for `origin`, or pass `--remote NAME` to configure another remote.
SSH uses the selected Gerrit username when available. For HTTPS, configure Git
to use your Gerrit HTTP password.

## Clone a project

`mw gerrit clone mediawiki/extensions/WSOAuth` fetches over HTTPS, configures
SSH for pushes, and installs the `commit-msg` hook. Pass
`--push-protocol https` to use HTTPS for both fetch and push.

## Push for review

`mw gerrit push` uploads the current commit stack to `refs/for/<branch>` on the
checkout's effective push remote. Use `--dry-run` to list outgoing commits and
matching public Gerrit reviews without uploading. Use `--branch NAME` to choose
the target or `--remote NAME` to choose a remote. A tracking branch uses its upstream
when the fetch and push URLs identify the same project, even with different
remote names. If the upstream is ambiguous, specify `--branch`. An untracked
branch uses the project's default branch. With a detached HEAD, the command uses the target of a matching public Gerrit
patchset or a unique open review matching the commit's Change-Id.

`mw gerrit push --https --profile NAME` selects HTTPS and uses the saved Gerrit
HTTP password. A Gerrit HTTPS push URL also selects HTTPS. The helper supplies
credentials to that Git invocation without changing the repository configuration.

## Check out a change

From a clean Git checkout of the project, run `mw gerrit checkout 1370181`
to check out the latest patchset as a detached HEAD. Pass `--patchset 4` to
select an older revision, or pass a Gerrit URL for that patchset. The command
requires a matching Git remote and works without a Gerrit account for public
changes. To clone missing repos or apply dependencies across a MediaWiki
development environment, use `mw dev mediawiki apply-patches`.

## Find and inspect reviews

Use `mw gerrit list --mine` for your open changes and `--reviewing` for open
changes where you are a reviewer. Both use the selected Gerrit profile.
Pass a Gerrit search query to `mw gerrit list` to search public changes.

`mw gerrit view 1370181` shows the patchset, current votes, submit requirements,
and unresolved threads. Run `mw gerrit diff 1370181 --base-patchset 3 --patchset 4`
to compare two patchsets, or omit `--base-patchset` to diff against the first
parent. Both commands accept a Gerrit change URL and infer the review and
patchset from Git HEAD when you omit the change.

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
