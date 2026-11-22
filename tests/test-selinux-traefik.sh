#!/usr/bin/env bash
# Exercise the production socket proxy against Podman's socket on Fedora.
set -euo pipefail

repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT

if [[ "${GITHUB_ACTIONS:-}" == true ]]; then
    sudo apt-get update -q
    sudo apt-get install -y --no-install-recommends qemu-system-x86 genisoimage python3-yaml
fi

image_name=Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2
image="${FEDORA_CLOUD_IMAGE:-$work/$image_name}"
if [[ -z "${FEDORA_CLOUD_IMAGE:-}" ]]; then
    curl --fail --location --silent --show-error --retry 3 --max-time 480         "https://dl.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/$image_name"         --output "$image"
fi
image=$(realpath "$image")
echo "28680fe5b371a5a82ebf43a31926e086a168e59949d03969c5093e7071f90b7f  $image" | sha256sum --check
qemu-img create -q -f qcow2 -F qcow2 -b "$image" "$work/guest.qcow2" 10G

# Keep the VM independent of registry access. Use the same image and flags
# as Compose, with fully qualified Docker Hub names for Podman.
python3 - "$repo_dir/mount/dev/compose/traefik/compose.yml" > "$work/proxy.env" <<'PYTHON'
import shlex
import sys

import yaml

with open(sys.argv[1]) as compose:
    proxy = yaml.safe_load(compose)["services"]["docker-socket-proxy"]

args = []
for option, field in [
    ("--security-opt", "security_opt"),
    ("--env", "environment"),
    ("--volume", "volumes"),
]:
    for value in proxy.get(field, []):
        args.extend([option, value])
image = proxy["image"]
if "." not in image.split("/")[0]:
    image = f"docker.io/{image}"
print(f"proxy_image={shlex.quote(image)}")
print(f"proxy_args=({shlex.join(args)})")
PYTHON
# shellcheck disable=SC1091
source "$work/proxy.env"
docker pull --platform linux/amd64 "$proxy_image"
docker save --output "$work/proxy.tar" "$proxy_image"

{
    cat <<'GUEST'
#!/bin/bash
set -euo pipefail
exec > /run/selinux-proxy.log 2>&1

finish() {
    cat /run/selinux-proxy.log > /dev/ttyS0
    systemctl poweroff --no-block
}
trap finish EXIT

getenforce | grep -x Enforcing
test "$(podman info --format '{{.Host.Security.SELinuxEnabled}}')" = true
systemctl start podman.socket
ln -s /run/podman/podman.sock /run/docker.sock
podman network create --internal gateway-control
mkdir /run/mwcli-ci
mount -o ro /dev/disk/by-label/CIDATA /run/mwcli-ci
podman load --input /run/mwcli-ci/proxy.tar
GUEST
    cat "$work/proxy.env"
    cat <<'GUEST'
podman run -d --pull=never --name docker-socket-proxy --network gateway-control "${proxy_args[@]}" "$proxy_image"
GUEST
    cat <<'GUEST'

# The container list must cross HAProxy and the mounted Unix socket.
for ((attempt = 0; attempt < 30; attempt++)); do
    if podman exec docker-socket-proxy wget -T 3 -qO- http://127.0.0.1:2375/containers/json > /tmp/proxy-containers.json &&
        python3 -c 'import json, sys; assert any(name.lstrip("/") == "docker-socket-proxy" for c in json.load(sys.stdin) for name in c["Names"])' < /tmp/proxy-containers.json; then
        echo 'SELinux socket proxy check passed'
        exit 0
    fi
    sleep 1
done
podman logs docker-socket-proxy || true
journalctl -k --no-pager -g 'avc:.*denied' -n 30 || true
exit 1
GUEST
} > "$work/user-data"
printf 'instance-id: mwcli-selinux-ci\n' > "$work/meta-data"
genisoimage -quiet -output "$work/seed.iso" -volid CIDATA -joliet -rock "$work/user-data" "$work/meta-data" "$work/proxy.tar"

# Use KVM when offered by the runner, with software emulation as a fallback.
timeout --kill-after=30s 20m qemu-system-x86_64     -machine accel=kvm:tcg -m 3072 -smp 2 -nographic -no-reboot     -drive "file=$work/guest.qcow2,if=virtio,format=qcow2"     -drive "file=$work/seed.iso,media=cdrom,readonly=on"     -nic user,model=virtio-net-pci,restrict=on | tee "$work/console.log"
grep -Fq 'SELinux socket proxy check passed' "$work/console.log"
