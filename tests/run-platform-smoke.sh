#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
source tests/functions.sh
cli=$(test_cli_binary "${MW_BIN:-}")
snapshot="${MWCLI_SMOKE_SNAPSHOT:?provide the MediaWiki CI snapshot archive}"
export MWCLI_CONTEXT_TEST=1
if [[ $# == 1 ]]; then
    exec bash tests/test-selinux-traefik.sh "$1"
fi

"$cli" --no-interaction update --dry-run
echo 'PASS: checked for a CLI update'

case "$(uname -s)" in
    Darwin)
        bash tests/test-macos-sudo-update.sh
        brew install colima docker docker-compose qemu
        mkdir -p "$HOME/.docker/cli-plugins"
        ln -sf "$(brew --prefix docker-compose)/bin/docker-compose" "$HOME/.docker/cli-plugins/docker-compose"
        colima start --runtime docker --vm-type qemu --mount-type sshfs --mount "$HOME:w"
        export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock
        export MWCLI_SMOKE_STARTUP_TIMEOUT=600
        docker info --format '{{.OSType}}' | grep -x linux
        bash tests/test-platform-stack.sh "$cli" "$snapshot"
        ;;
    MINGW*|MSYS*)
        bash tests/test-windows-update.sh
        export MSYS2_ARG_CONV_EXCL='*'
        distro=Ubuntu-24.04
        if ! wsl.exe --distribution "$distro" --exec true; then
            wsl.exe --install --distribution "$distro" --web-download --no-launch
        fi
        # Reuse the GitLab Linux binary and run it beside the daemon so bind mounts use WSL paths.
        wsl.exe --distribution "$distro" --user root --exec bash -s -- "$snapshot" <<'WSL'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -y --no-install-recommends docker.io docker-compose-v2 curl
service docker start
docker info --format '{{.OSType}}' | grep -x linux
bin_dir=$(mktemp -d)
trap 'rm -rf -- "$bin_dir"' EXIT
install -m 0755 ../inputs/mw "$bin_dir/mw"
export MWCLI_CONTEXT_TEST=1
"$bin_dir/mw" --no-interaction update --dry-run
echo 'PASS: checked for a CLI update inside WSL'
bash tests/test-platform-stack.sh "$bin_dir/mw" "$1"
WSL
        ;;
    *)
        bash tests/test-platform-stack.sh "$cli" "$snapshot"
        ;;
esac
