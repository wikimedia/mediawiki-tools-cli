#!/usr/bin/env bash
# Run locally as root on the runner, or upload with cloudvps compute ssh --script.
set +x
set -euo pipefail

configure_docker() {
    python3 - "${1:-}" <<'PY'
import json
import os
import sys
import tempfile
from pathlib import Path

path = Path("/etc/docker/daemon.json")
config = json.loads(path.read_text()) if path.exists() else {}
# Keep images on the mounted Docker volume, including reused overlay2 data.
config.setdefault("features", {})["containerd-snapshotter"] = False
if sys.argv[1]:
    mirrors = config.setdefault("registry-mirrors", [])
    if sys.argv[1] not in mirrors:
        mirrors.insert(0, sys.argv[1])
path.parent.mkdir(parents=True, exist_ok=True)
with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as stream:
    json.dump(config, stream, indent=2)
    stream.write("\n")
    temporary = stream.name
os.replace(temporary, path)
PY
}

runner_config() {
    python3 - "$@" <<'PY'
import os
import sys
import tempfile
from pathlib import Path
import tomlkit

path = Path("/etc/gitlab-runner/config.toml")
action, name = sys.argv[1:3]
original = path.read_text() if path.exists() else ""
config = tomlkit.parse(original)
matches = [r for r in config.get("runners", []) if r.get("name") == name]
if len(matches) > 1:
    raise SystemExit(f"Duplicate runner entries named {name}; reconcile config.toml first")
if action == "count":
    print(len(matches))
    sys.exit()
if not matches:
    raise SystemExit(f"Runner {name} is not registered locally")
runner = matches[0]
if runner.get("url", "").rstrip("/") != "https://gitlab.wikimedia.org":
    raise SystemExit("Existing runner points to a different GitLab instance")
if runner.get("executor") != "docker":
    raise SystemExit("Existing runner does not use the Docker executor")
config["concurrent"] = 2
runner["limit"] = 2
docker = runner.setdefault("docker", tomlkit.table())
docker["pull_policy"] = ["always", "if-not-present"]
services = docker.setdefault("services", tomlkit.aot())
matches = [s for s in services if s.get("name") == "docker:26.1.1-dind"]
if len(matches) > 1:
    raise SystemExit("Duplicate docker:26.1.1-dind service entries; reconcile before continuing")
if matches:
    service = matches[0]
else:
    service = tomlkit.table()
    service["name"] = "docker:26.1.1-dind"
    services.append(service)
service["command"] = ["--registry-mirror", sys.argv[3]]
updated = tomlkit.dumps(config)
if updated != original:
    backup = path.with_name(path.name + ".before-mwcli")
    if not backup.exists():
        # Create with restricted permissions before copying the runner token.
        with open(backup, "x", opener=lambda p, flags: os.open(p, flags, 0o600)) as stream:
            stream.write(original)
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as stream:
        stream.write(updated)
        temporary = stream.name
    os.replace(temporary, path)
os.chmod(path, 0o600)
PY
}

install_packages() {
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y ca-certificates curl python3 python3-tomlkit
    configure_docker
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
        apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    fi
    systemctl daemon-reload
    systemctl enable docker
    systemctl restart docker
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

configure_runner() {
    local ipv4 mirror name count token username password
    ipv4=$(ip -4 route get 1.1.1.1 | awk '{for (i=1; i<=NF; i++) if ($i=="src") {print $(i+1); exit}}')
    python3 -c 'import ipaddress,sys; ipaddress.IPv4Address(sys.argv[1])' "$ipv4"
    mirror="http://${ipv4}:6000"
    name="$(hostname -s)-docker"

    if docker container inspect registry >/dev/null 2>&1; then
        echo "Reusing the existing registry container and its credentials."
        docker start registry >/dev/null
    else
        IFS= read -r -p "Docker Hub username: " username </dev/tty
        IFS= read -rs -p "Docker Hub read-only token: " password </dev/tty
        printf '\n' >/dev/tty
        [[ -n $username && -n $password ]] || { echo "Docker Hub credentials are required." >&2; return 1; }
        REGISTRY_PROXY_USERNAME="$username" REGISTRY_PROXY_PASSWORD="$password" \
            docker run -d --name registry --restart always -p "${ipv4}:6000:5000" \
            -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
            -e REGISTRY_PROXY_USERNAME -e REGISTRY_PROXY_PASSWORD \
            -v mwcli-registry:/var/lib/registry registry:2
        unset password
    fi
    curl -fsS --retry 10 --retry-connrefused --retry-delay 1 --max-time 10 "$mirror/v2/" >/dev/null
    configure_docker "$mirror"
    systemctl restart docker

    count=$(runner_config count "$name")
    systemctl stop gitlab-runner
    if [[ $count == 0 ]]; then
        echo "Create the project runner in GitLab first (tag mwcli, untagged jobs off, locked to project)."
        IFS= read -rs -p "GitLab runner authentication token (glrt-...): " token </dev/tty
        printf '\n' >/dev/tty
        [[ $token == glrt-* ]] || { echo "Expected a glrt- runner authentication token." >&2; return 1; }
        # Fixed Docker defaults belong here; later updates only adjust the mirror and limits.
        CI_SERVER_TOKEN="$token" gitlab-runner register --non-interactive \
            --url https://gitlab.wikimedia.org/ --executor docker \
            --name "$name" --docker-image docker:26.1.1 \
            --docker-privileged --docker-volumes /certs/client
        unset token
    else
        echo "Updating the existing runner $name."
    fi
    runner_config update "$name" "$mirror"
    systemctl enable --now gitlab-runner
    gitlab-runner verify
}

status() {
    findmnt /var/lib/docker
    df -h /var/lib/docker
    docker info --format 'Storage: {{.Driver}}; Root: {{.DockerRootDir}}; Mirrors: {{json .RegistryConfig.Mirrors}}'
    docker ps -a --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
    if command -v gitlab-runner >/dev/null; then
        gitlab-runner status
    fi
}

action=${1:-help}
case $action in
    help|--help|-h)
        echo "Usage: runner.sh {install|configure|setup|status}"
        echo "install: install Docker and GitLab Runner; preserve existing Docker data."
        echo "configure: reuse/create the mirror, register/configure the runner, and verify."
        echo "setup: install, then configure (prompts for missing credentials)."
        echo "status: show mount, storage, containers, and runner service status."
        exit 0
        ;;
    install|configure|setup|status) ;;
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
    configure) configure_runner ;;
    setup) install_packages; configure_runner ;;
esac
status
