#!/usr/bin/env bash
# Run locally as root on the runner, or upload with cloudvps compute ssh --script.
set +x
set -euo pipefail

install_packages() {
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y ca-certificates curl python3
    mkdir -p /etc/systemd/system/docker.service.d
    cat > /etc/systemd/system/docker.service.d/mwcli-volume.conf <<'EOF'
[Unit]
RequiresMountsFor=/var/lib/docker
EOF
    if ! command -v docker >/dev/null; then
        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
        chmod a+r /etc/apt/keyrings/docker.asc
        cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/debian
Suites: trixie
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF
        apt-get update
        apt-get install -y docker-ce docker-ce-cli containerd.io
    fi
    systemctl daemon-reload
    systemctl enable --now docker
    if ! command -v gitlab-runner >/dev/null; then
        (
            installer=$(mktemp)
            trap 'rm -f "$installer"' EXIT
            curl -fsSL https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh -o "$installer"
            bash "$installer"
            NO_DOCKER_PRUNE=1 apt-get install -y gitlab-runner
        )
    fi
}

status() {
    findmnt /var/lib/docker
    df -h /var/lib/docker
    docker info --format 'Storage: {{.Driver}}; Root: {{.DockerRootDir}}'
    gitlab-runner verify --config /etc/gitlab-runner-wait/config.toml
    systemctl --no-pager status gitlab-runner-wait mwcli-wait-status
}

action=${1:-help}
case $action in
    help|--help|-h)
        echo "Usage: runner.sh {install|status}"
        echo "install: install Docker and GitLab Runner; preserve existing Docker data."
        echo "Register and configure waiting pools using CI.md."
        echo "status: show storage, registration, and waiting service status."
        exit 0
        ;;
    install|status) ;;
    *) echo "Unknown action: $action" >&2; exit 1 ;;
esac

[[ $(id -u) == 0 ]] || { echo "Run as root (use --sudo)." >&2; exit 1; }
# shellcheck disable=SC1091
source /etc/os-release
[[ $ID == debian && $VERSION_ID == 13 ]] || {
    echo "This runner setup supports Debian 13 (Trixie)." >&2; exit 1;
}
mountpoint -q /var/lib/docker || {
    echo "Mount the data volume at /var/lib/docker first; this script never formats disks." >&2
    exit 1
}

case $action in
    install) install_packages ;;
    status) status ;;
esac
