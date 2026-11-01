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

### Artifact storage overview

Use a compact summary to find the largest artifact consumers before choosing
prune filters:

```sh
mw gitlab ci artifacts summary
mw gitlab ci artifacts summary -R releng/cli
mw gitlab ci artifacts summary --since 7d --ref-prefix refs/merge-requests/
mw gitlab ci artifacts summary --output json
```

The summary is read-only and defaults to the entire project job history
(`--since all`), following all pages including retried jobs. Large projects can
take time to scan; `--since`, `--ref-prefix`, and `--min-size` narrow the overview.
Totals reflect only matching artifacts visible to your account at scan time,
not a transactional snapshot or the project's total disk usage.

Text output shows total bytes, artifact count, and distinct job count, followed
by separate job-name and artifact-type tables sorted by decreasing storage size.
Repeated runs of the same job name are combined. Job counts in the type table
can overlap because a job can have multiple types. JSON returns `total`,
`by_job`, and `by_type`, with `jobs`, `artifacts`, and exact `size_bytes` fields
for each entry; grouped entries also have a `name`.

GitLab's Jobs API reports sizes for stored artifact files such as `archive`,
`metadata`, and `junit`. The summary counts each entry in the `artifacts` array
once, not the duplicate archive total in `artifacts_file`. Archive metadata is a
separate stored file, not a second measurement of the archive. Job logs (`trace`)
are excluded, just as in list/prune. Types describe GitLab storage categories,
not file extensions or uncompressed archive contents; the API does not expose
per-file sizes inside an archive here. A missing type is shown as `unknown`.

## Find repositories

```sh
mw gitlab repo search --search="docker"
```

Use `mw gitlab --help` or a subcommand's `--help` for all options. The [command reference](https://a-wikimedia-cli.toolforge.org/manual/gitlab/) documents the version included in this release.
