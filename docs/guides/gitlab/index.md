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

## Find repositories

```sh
mw gitlab repo search --search="docker"
```

Use `mw gitlab --help` or a subcommand's `--help` for all options. The [command reference](https://www.mediawiki.org/wiki/Cli/ref/mw_gitlab) documents the version included in this release.
