<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Update?oldid=5784907 (CC BY-SA 4.0); see the page history for contributors. -->

# Keep the CLI up to date

Check your current version and preview an update without replacing the executable:

```sh
mw version
mw update --dry-run
```

Install the latest release:

```sh
mw update
```

Use `--dry-run` to see the selected release, download location and release notes without replacing the installed binary. To preview a specific release:

```sh
mw update --dry-run --version v0.33.0
```

The updater shows the changelog between your current and target versions. It does not change your installation during a dry run.

The updater asks before replacing the executable. Use `sudo mw update` only if the executable is installed in a directory your user cannot write to. Package-managed installations should follow their package manager's update procedure.

To install a specific published version, use `mw update --version VERSION` with a version shown on the [releases page](https://gitlab.wikimedia.org/repos/releng/cli/-/releases).

## Release versions

The updater supports existing `v0.x.y` releases and future date-based tags in
`vYYYY.MM.DD.REVISION` format. For example, `v2026.10.31.0` identifies an initial
release and `v2026.10.31.1` a maintenance update to that release series, even if
the maintenance update is published later. Dates must be valid calendar dates;
months and days can have leading zeroes. The revision starts at zero and increases
numerically. Dated prereleases are not currently supported.

Ordinary `mw update` selects the highest stable version, rather than the most
recently published release. An older series receiving a backport will not replace
a newer installed series. Dated versions sort above existing semantic versions.
Prerelease tags, unrecognised versions, and releases with a future publication
date are excluded from automatic selection. Nightly selection remains separate.

`--force` allows reinstalling the selected version; it does not enable automatic
downgrades. To deliberately install an older release, select it explicitly:

```sh
mw update --version v2026.10.31.1
```

Support for dated versions can ship in any number of `v0.x.y` releases before
the project switches formats. Older binaries without this support cannot compare
four-part versions in update notifications; upgrade to a version with this support
before relying on automatic dated-version checks.

## Nightly and CI builds

Once nightly builds are configured, preview or install the latest successful
nightly built from `main`:

```sh
mw update --version nightly --dry-run
mw update --version nightly
```

This finds the newest successful **Nightly CLI build** pipeline on the existing
`scheduled` ref, then downloads only the executable for your OS and architecture.
Weekly and Dependabot pipelines on that same ref are ignored. If no successful
nightly exists, or its artifact has expired, the command stops without replacing
your executable. Nightlies are development builds; ordinary `mw update` continues
to select published releases.

`--version` also accepts an HTTP(S) executable URL, a local executable path, or a
GitLab artifact ZIP URL. Archive support covers:

| Job output | Archive layout | Compatibility |
| --- | --- | --- |
| `build` | `bin/mw`, `bin/wm`, or their Windows `.exe` names | Runner-native binary only |
| `build-release` | `_release/VERSION/mw_VERSION_OS_ARCH` (or `wm_...`) | Selects your OS and architecture |
| `release-build` | Same layout, with a tagged version | Selects your OS and architecture |
| Nightly `build-release` | `_release/nightly/mw_nightly_OS_ARCH` (or `wm_...`) | Selects your OS and architecture |

For example, replace `JOB_ID` with the chosen job's ID to install its ZIP:

```sh
mw update --version 'https://gitlab.wikimedia.org/api/v4/projects/16/jobs/JOB_ID/artifacts'
```

For an older CLI that cannot select a binary from a multi-platform ZIP, use a
direct executable URL instead. For a Linux AMD64 nightly, that URL is:

```sh
mw update --version 'https://gitlab.wikimedia.org/api/v4/projects/16/jobs/JOB_ID/artifacts/_release/nightly/mw_nightly_linux_amd64'
```

Native and preview build artifacts expire after two days; tagged build artifacts
keep their separate release retention. GitLab's keep-latest setting can override
these expiry times. Published release packages are separate from CI artifacts.

Updates preserve the executable's installed name and location. A binary called `wm` stays `wm`; configuration and credentials stay in their existing directories.

See the [installation guide](installation.md) for a fresh installation, and the [command reference](https://www.mediawiki.org/wiki/Cli/ref/mw_update) for all update options.
