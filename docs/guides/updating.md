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

Updates preserve the executable's installed name and location. A binary called `wm` stays `wm`; configuration and credentials stay in their existing directories.

See the [installation guide](installation.md) for a fresh installation, and the [command reference](https://www.mediawiki.org/wiki/Cli/ref/mw_update) for all update options.
