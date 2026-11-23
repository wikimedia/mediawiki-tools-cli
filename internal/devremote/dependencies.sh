set -euo pipefail
if command -v git >/dev/null && command -v flock >/dev/null && command -v ss >/dev/null && docker compose version >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    exit 0
fi
. /etc/os-release
docker_packages=(docker.io)
case "$ID:$VERSION_ID" in
    ubuntu:24.04|ubuntu:26.04) compose=docker-compose-v2 ;;
    debian:13) compose=docker-compose; docker_packages+=(docker-cli) ;;
    *) echo 'Automatic dependency installation supports Ubuntu 24.04/26.04 and Debian 13. Install Git and Docker Compose manually on this VM.' >&2; exit 1 ;;
esac
sudo -n true || { echo 'Dependency installation requires passwordless sudo; install Docker manually instead.' >&2; exit 1; }
sudo -n apt-get update
packages=(git util-linux iproute2 ca-certificates curl rsync)
if ! command -v docker >/dev/null; then
    packages+=("${docker_packages[@]}" "$compose")
elif ! docker compose version >/dev/null 2>&1; then
    if apt-cache show docker-compose-plugin >/dev/null 2>&1; then
        compose=docker-compose-plugin
    fi
    packages+=("$compose")
fi
sudo -n env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-upgrade --no-install-recommends "${packages[@]}"
if ! docker info >/dev/null 2>&1; then
    if ! sudo -n docker info >/dev/null 2>&1; then
        sudo -n systemctl enable --now docker
    fi
    sudo -n usermod -aG docker "$(id -un)"
fi
# The next SSH session picks up the new Docker group membership.
