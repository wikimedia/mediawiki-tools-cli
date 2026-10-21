<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Gitlab?oldid=5065509 (CC BY-SA 4.0); see the page history for contributors. -->

# Work with GitLab

`mw gitlab` (alias `mw gl`) includes the glab CLI and defaults to Wikimedia's GitLab instance. Authenticate before running commands that need your account:

```sh
mw gitlab auth login
```

Run repository commands from a checkout with a GitLab remote.

## Merge requests

```sh
mw gitlab mr list
mw gitlab mr create --fill
```

## Releases and pipelines

```sh
mw gitlab release list
mw gitlab ci lint
mw gitlab ci status
```

List individual artifacts created by recent jobs in the current project, or
select another project with `-R`:

```sh
mw gitlab ci artifacts list --since 48h --min-size 300MiB
mw gitlab ci artifacts list --since 7d --min-size 1GiB -R releng/cli
mw gitlab ci artifacts list --since 24h --min-size 300MiB --ref-prefix refs/merge-requests/
mw gitlab ci artifacts prune --since 24h --min-size 300MiB --ref-prefix refs/merge-requests/
```

Listing scans recent jobs and shows individual artifact files and their sizes.
The prune command without `--yes` shows the matching files and stops without
deleting them. Add `--yes` to delete those individual files; job logs are not
included. GitLab marks bulk artifact deletion as experimental.

The [artifact recording](artifacts.tape) demonstrates listing merge request
artifacts from the last 24 hours and previewing the same set for pruning.

## Find repositories

```sh
mw gitlab repo search --search="docker"
```

Use `mw gitlab --help` or a subcommand's `--help` for all options. The [command reference](https://www.mediawiki.org/wiki/Cli/ref/mw_gitlab) documents the version included in this release.
