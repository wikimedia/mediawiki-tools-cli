<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Installation?oldid=8605285 (CC BY-SA 4.0); see the page history for contributors. -->

# Install a Wikimedia CLI

Download the `wm_` binary and matching `.sha256` file for your operating system and CPU from the [latest release](https://gitlab.wikimedia.org/repos/releng/cli/-/releases/permalink/latest), or use the commands below. On Windows, save the `.exe` asset as `wm.exe`. For the development environment, use a Linux installation inside WSL2.

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
UA='a-wikimedia-cli-docs/1.0 (https://a-wikimedia-cli.toolforge.org/)'
RELEASE_URL=$(curl -fsSL -A "$UA" -o /dev/null -w '%{url_effective}' \
  https://gitlab.wikimedia.org/repos/releng/cli/-/releases/permalink/latest)
VER=${RELEASE_URL##*/}
ASSET="wm_${VER}_${PLATFORM}_${ARCH}"
STATUS=$(curl -sSIL -A "$UA" -o /dev/null -w '%{http_code}' "$RELEASE_URL/downloads/binaries/$ASSET")
case "$STATUS" in
  200) ;;
  404) ASSET="mw_${VER}_${PLATFORM}_${ARCH}" ;;
  *) echo "Could not check the release asset (HTTP $STATUS)" >&2; exit 1 ;;
esac
curl -fL -A "$UA" "$RELEASE_URL/downloads/binaries/$ASSET" -o wm
curl -fL -A "$UA" "$RELEASE_URL/downloads/binaries/$ASSET.sha256" -o wm.sha256
printf '%s  wm\n' "$(cat wm.sha256)" | shasum -a 256 -c -
)
```

The script discovers the release version and can use an older `mw_` asset if the `wm_` asset is missing. A successful checksum check prints `wm: OK`. For other CPUs or manually downloaded files, save the binary as `wm` and the matching checksum as `wm.sha256`, then verify them before installing:

```sh
printf '%s  wm\n' "$(cat wm.sha256)" | shasum -a 256 -c -
```

For an installation in your home directory:

```sh
mkdir -p "$HOME/.local/bin"
install -m 0755 wm "$HOME/.local/bin/wm"
```

Add `$HOME/.local/bin` to your shell's `PATH` if needed. To install for all users instead, run `sudo install -m 0755 wm /usr/local/bin/wm`.

Check the installation:

```sh
wm version
wm --help
```

After checking the installation, remove the downloaded copies from your working directory:

```sh
rm wm wm.sha256
```

## Community packages

Arch Linux users can also use the community-maintained [mwcli package in the Arch User Repository (AUR)](https://aur.archlinux.org/packages/mwcli). Community packages may lag behind the latest upstream release. For package-managed installations, follow your package manager's update procedure.

## Command names and configuration

The default command is `wm`. Help and completion use the name you invoke, and both names share the existing `mwcli` configuration and saved credentials.

If you already have `mw`, run `mw update` to update that installation in place. Scripts and MCP clients can keep using its existing path. On Linux or macOS, you can add `wm` as a symlink to the same executable. If `mw` is installed in `$HOME/.local/bin`, run:

```sh
ln -s mw "$HOME/.local/bin/wm"
```

Docker is needed for the [development environment](dev/setup.md); the commands for Wikimedia data and remote services do not require it.

Continue with [updates](updating.md) or [your first local wiki](dev/setup.md).
