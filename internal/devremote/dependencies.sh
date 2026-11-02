set -euo pipefail
if command -v git >/dev/null && command -v flock >/dev/null && command -v ss >/dev/null && docker compose version >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    exit 0
fi
. /etc/os-release
case "$ID:$VERSION_ID" in
    ubuntu:24.04|ubuntu:26.04) compose=docker-compose-v2 ;;
    debian:13) compose=docker-compose ;;
    *) echo 'Automatic dependency installation supports Ubuntu 24.04/26.04 and Debian 13. Install Git and Docker Compose manually on this VM.' >&2; exit 1 ;;
esac
sudo -n true || { echo 'Dependency installation requires passwordless sudo; install Docker manually instead.' >&2; exit 1; }
sudo -n apt-get update
sudo -n env DEBIAN_FRONTEND=noninteractive apt-get install -y git docker.io "$compose" util-linux iproute2 ca-certificates curl rsync
sudo -n systemctl enable --now docker
sudo -n usermod -aG docker "$(id -un)"
# The next SSH session picks up the new Docker group membership.
