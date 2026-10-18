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

The updater asks before replacing the executable. Use `sudo mw update` only if the executable is installed in a directory your user cannot write to. Package-managed installations should follow their package manager's update procedure.

To install a specific published version, use `mw update --version VERSION` with a version shown on the [releases page](https://gitlab.wikimedia.org/repos/releng/cli/-/releases).

Updates preserve the executable's installed name and location. A binary called `wm` stays `wm`; configuration and credentials stay in their existing directories.

See the [installation guide](installation.md) for a fresh installation, and the [command reference](https://www.mediawiki.org/wiki/Cli/ref/mw_update) for all update options.
