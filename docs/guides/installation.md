<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Installation?oldid=8605285 (CC BY-SA 4.0); see the page history for contributors. -->

# Install a Wikimedia CLI

Download the binary and matching `.sha256` file for your operating system and CPU from the [latest release](https://gitlab.wikimedia.org/repos/releng/cli/-/releases/permalink/latest). Release assets currently use the `mw_` prefix. On Windows, use the `.exe` asset; for the development environment, use a Linux installation inside WSL2.

## Verify and install on Linux or macOS

Save the downloaded binary as `mw` and its checksum as `mw.sha256`. Verify the checksum before installing:

```sh
printf '%s  mw\n' "$(cat mw.sha256)" | shasum -a 256 -c -
```

For an installation in your home directory:

```sh
mkdir -p "$HOME/.local/bin"
install -m 0755 mw "$HOME/.local/bin/mw"
```

Add `$HOME/.local/bin` to your shell's `PATH` if needed. To install for all users instead, run `sudo install -m 0755 mw /usr/local/bin/mw`.

Check the installation:

```sh
mw version
mw --help
```

## Command names and configuration

The project is called **a Wikimedia CLI**. The default command remains `mw`; the same executable also works when renamed or symlinked as `wm`. Help and completion use the name you invoke. Both names use the existing `mwcli` configuration directory and saved credentials.

Docker is needed for the [development environment](mediawiki.md); the commands for Wikimedia data and remote services do not require it.

Continue with [updates](updating.md) or [your first local wiki](mediawiki.md).
