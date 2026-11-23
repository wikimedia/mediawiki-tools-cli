#!/usr/bin/env bash
# Run on a disposable Linux runner VM and keep the engine scoped to its user.
set -euo pipefail
engine=$1
shift

sudo loginctl enable-linger "$(id -un)"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
export DBUS_SESSION_BUS_ADDRESS="${DBUS_SESSION_BUS_ADDRESS:-unix:path=$XDG_RUNTIME_DIR/bus}"
unset DOCKER_CONTEXT
case $engine in
    docker-rootless)
        packages=(uidmap dbus-user-session slirp4netns)
        sudo apt-get update
        if ! command -v dockerd-rootless-setuptool.sh >/dev/null; then
            # Check refreshed repository indexes. The runner can have stale
            # Docker metadata even after its APT source has been removed.
            version=$(dpkg-query -W -f='${Version}' docker-ce)
            if ! apt-cache madison docker-ce-rootless-extras |
                awk -v version="$version" '$3 == version {found=1} END {exit !found}'; then
                . /etc/os-release
                sudo install -d -m 0755 /etc/apt/keyrings
                curl -fsSL --proto '=https' https://download.docker.com/linux/ubuntu/gpg |
                    sudo tee /etc/apt/keyrings/mw-ci-docker.asc >/dev/null
                sudo chmod 0644 /etc/apt/keyrings/mw-ci-docker.asc
                echo "deb [signed-by=/etc/apt/keyrings/mw-ci-docker.asc] https://download.docker.com/linux/ubuntu $VERSION_CODENAME stable" |
                    sudo tee /etc/apt/sources.list.d/mw-ci-docker.list >/dev/null
                sudo apt-get update
            fi
            packages+=("docker-ce-rootless-extras=$version")
        fi
        sudo apt-get install -y "${packages[@]}"
        dockerd-rootless-setuptool.sh install --force
        export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/docker.sock"
        docker info --format '{{range .SecurityOptions}}{{println .}}{{end}}' | grep -Fx name=rootless
        ;;
    podman)
        sudo apt-get update
        sudo apt-get install -y uidmap podman
        systemctl --user start podman.socket
        export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"
        test "$(podman info --format '{{.Host.Security.Rootless}}')" = true
        docker info --format '{{range .SecurityOptions}}{{println .}}{{end}}' | grep -Fx name=rootless
        ;;
    *)
        echo "Unsupported CI engine: $engine" >&2
        exit 1
        ;;
esac
exec "$@"
