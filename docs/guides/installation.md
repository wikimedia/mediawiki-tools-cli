<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Installation?oldid=8605285 (CC BY-SA 4.0); see the page history for contributors. -->

# Install a Wikimedia CLI

Download the binary and matching `.sha256` file for your operating system and CPU from the [latest release](https://gitlab.wikimedia.org/repos/releng/cli/-/releases/permalink/latest), or use the commands below. Release assets currently use the `mw_` prefix. On Windows, use the `.exe` asset; for the development environment, use a Linux installation inside WSL2.

## Verify and install on Linux or macOS

In an empty directory, download and verify the latest release for Linux or macOS on an Intel/AMD 64-bit or ARM64 machine:

```sh
(
set -e
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  x86_64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo 'Choose a binary for your CPU from the releases page.' >&2; exit 1 ;;
esac
UA='a-wikimedia-cli (https://www.mediawiki.org/wiki/Cli)'
RELEASE_URL=$(curl -fsSL -A "$UA" -o /dev/null -w '%{url_effective}' \
  https://gitlab.wikimedia.org/repos/releng/cli/-/releases/permalink/latest)
VER=${RELEASE_URL##*/}
ASSET="mw_${VER}_${PLATFORM}_${ARCH}"
curl -fL -A "$UA" "$RELEASE_URL/downloads/binaries/$ASSET" -o mw
curl -fL -A "$UA" "$RELEASE_URL/downloads/binaries/$ASSET.sha256" -o mw.sha256
printf '%s  mw\n' "$(cat mw.sha256)" | shasum -a 256 -c -
)
```

The release version is discovered automatically. A successful checksum check prints `mw: OK`. For other CPUs or manually downloaded files, save the binary as `mw` and the matching checksum as `mw.sha256`, then verify them before installing:

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

After checking the installation, remove the downloaded copies from your working directory:

```sh
rm mw mw.sha256
```

## Community packages

Arch Linux users can also use the community-maintained [mwcli package in the Arch User Repository (AUR)](https://aur.archlinux.org/packages/mwcli). Community packages may lag behind the latest upstream release. For package-managed installations, follow your package manager's update procedure.

## Command names and configuration

The project is called **a Wikimedia CLI**. The default command remains `mw`; the same executable also works when renamed or symlinked as `wm`. Help and completion use the name you invoke. Both names use the existing `mwcli` configuration directory and saved credentials.

Docker is needed for the [development environment](dev/setup.md); the commands for Wikimedia data and remote services do not require it.

Continue with [updates](updating.md) or [your first local wiki](dev/setup.md).
