#!/usr/bin/env bash
# Exercise the development gateway and a PHP bind mount on enforcing Fedora.
set -euo pipefail

repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_dir"
source tests/functions.sh
mode=${1:-rootful}
case "$mode" in
    rootful|rootless) ;;
    *) echo "Invalid Podman mode: $mode" >&2; exit 2 ;;
esac
cli=$(test_cli_binary "${MW_BIN:-}")
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT

if [[ "${GITHUB_ACTIONS:-}" == true ]]; then
    sudo apt-get update -q
    sudo apt-get install -y --no-install-recommends qemu-system-x86 genisoimage
fi

image_name=Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2
image="${FEDORA_CLOUD_IMAGE:-$work/$image_name}"
if [[ -z "${FEDORA_CLOUD_IMAGE:-}" ]]; then
    curl --fail --location --silent --show-error --retry 3 --max-time 480 \
        "https://dl.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/$image_name" \
        --output "$image"
fi
image=$(realpath "$image")
echo "28680fe5b371a5a82ebf43a31926e086a168e59949d03969c5093e7071f90b7f  $image" | sha256sum --check
qemu-img create -q -f qcow2 -F qcow2 -b "$image" "$work/guest.qcow2" 10G

# Pass the tested CLI, Compose client and production images into the offline VM.
install -m 0755 "$cli" "$work/cli"
install -m 0755 "$(command -v docker)" "$work/docker"
compose_plugin=$(docker info --format '{{range .ClientInfo.Plugins}}{{if eq .Name "compose"}}{{.Path}}{{end}}{{end}}')
install -m 0755 "$compose_plugin" "$work/docker-compose"

# Read the image references with Compose so they track the embedded definitions.
compose_dir="$repo_dir/mount/dev"
PORT=8080 NETWORK_SUBNET_PREFIX=10.253.44 TRAEFIK_DOCKER_NETWORK=fixture_dps \
    TRAEFIK_DOCKER_PROJECT=fixture MEDIAWIKI_VOLUMES_CODE=/root/selinux-wiki SHELLBOX_SECRET_KEY=fixture \
    docker compose --project-directory "$compose_dir" --profile gateway \
    -f "$compose_dir/compose/base/compose.yml" -f "$compose_dir/compose/traefik/compose.yml" \
    -f "$compose_dir/compose/mediawiki/compose.yml" config --images > "$work/images.txt"
mapfile -t images < "$work/images.txt"
for container_image in "${images[@]}"; do
    docker pull --platform linux/amd64 "$container_image"
done
docker save --output "$work/images.tar" "${images[@]}"

cat > "$work/stack.sh" <<'STACK'
#!/bin/bash
set -euo pipefail
mode=$1
test "$(podman info --format '{{.Host.Security.SELinuxEnabled}}')" = true
expected_rootless=false
if [[ "$mode" == rootless ]]; then
    expected_rootless=true
    test "$(id -u)" -ne 0
fi
test "$(podman info --format '{{.Host.Security.Rootless}}')" = "$expected_rootless"
echo "Podman mode: $mode"
podman load --input /run/mwcli-ci/images.tar

cd "$HOME"
mkdir selinux-wiki
printf '%s\n' '<?php echo "SELinux PHP bind mount works";' > selinux-wiki/probe.php
mw dev env set PORT 8080
mw dev env set NETWORK_SUBNET_PREFIX 10.253.44
mw dev env set MEDIAWIKI_VOLUMES_CODE "$HOME/selinux-wiki"
if [[ "$mode" == rootless ]]; then
    mw dev env set MWCLI_DOCKER_SOCKET "${DOCKER_HOST#unix://}"
fi
mw dev traefik create --no-interaction
mw dev compose -- up -d --no-build --pull never mediawiki gateway-auth
port=$(mw dev env get PORT)
response=$(mktemp)
trap 'rm -f -- "$response"' EXIT

probe() {
    local url=$1 expected=$2
    for ((attempt = 0; attempt < 60; attempt++)); do
        if curl --noproxy '*' --max-time 3 -fsS "$url" > "$response" &&
            grep -Fq -- "$expected" "$response"; then
            printf 'PASS %s\n' "$url"
            return 0
        fi
        sleep 2
    done
    mw dev compose -- ps
    mw dev compose -- logs --tail 35 traefik docker-socket-proxy dashboard mediawiki mediawiki-web gateway-auth
    journalctl -k --no-pager -g 'avc:.*denied' -n 30 || true
    return 1
}

probe "http://127.0.0.1:$port/proxy/dashboard/health" '"status":"ok"'
probe "http://127.0.0.1:$port/proxy/selinux-ci/w/probe.php" 'SELinux PHP bind mount works'
probe "http://127.0.0.1:$port/_wm/login" 'Sign in to the gateway'
probe "http://127.0.0.1:$port/_wm/assets/tokens.css" 'Codex Design Tokens'
probe "http://127.0.0.1:$port/proxy/dashboard/assets/tokens.css" 'Codex Design Tokens'
echo 'SELinux development gateway checks passed'
STACK

{
    printf '#!/bin/bash\nmode=%q\n' "$mode"
    cat <<'GUEST'
set -euo pipefail
exec > /run/selinux-dev.log 2>&1

finish() {
    cat /run/selinux-dev.log > /dev/ttyS0
    systemctl poweroff --no-block
}
trap finish EXIT

getenforce | grep -x Enforcing
mkdir /run/mwcli-ci
mount -o ro /dev/disk/by-label/CIDATA /run/mwcli-ci
install -Dm0755 /run/mwcli-ci/cli /usr/local/bin/mw
install -Dm0755 /run/mwcli-ci/docker /usr/local/bin/docker
install -Dm0755 /run/mwcli-ci/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose
export MWCLI_CONTEXT_TEST=1
if [[ "$mode" == rootless ]]; then
    uid=$(id -u fedora)
    systemctl start "user@$uid.service"
    runtime="/run/user/$uid"
    as_fedora=(runuser -u fedora -- env "XDG_RUNTIME_DIR=$runtime" "DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime/bus")
    "${as_fedora[@]}" systemctl --user start podman.socket
    "${as_fedora[@]}" "DOCKER_HOST=unix://$runtime/podman/podman.sock" bash /run/mwcli-ci/stack.sh "$mode"
else
    systemctl start podman.socket
    ln -s /run/podman/podman.sock /run/docker.sock
    HOME=/root DOCKER_HOST=unix:///run/podman/podman.sock bash /run/mwcli-ci/stack.sh "$mode"
fi
GUEST
} > "$work/user-data"
printf 'instance-id: mwcli-selinux-ci\n' > "$work/meta-data"
genisoimage -quiet -output "$work/seed.iso" -volid CIDATA -joliet -rock "$work/user-data" "$work/meta-data" "$work/stack.sh" "$work/images.tar" "$work/cli" "$work/docker" "$work/docker-compose"

# Use KVM when offered by the runner, with software emulation as a fallback.
vm_status=0
timeout --kill-after=30s 20m qemu-system-x86_64 \
    -machine accel=kvm:tcg -m 3072 -smp 2 -nographic -no-reboot \
    -drive "file=$work/guest.qcow2,if=virtio,format=qcow2" \
    -drive "file=$work/seed.iso,media=cdrom,readonly=on" \
    -nic user,model=virtio-net-pci,restrict=on > "$work/console.log" 2>&1 || vm_status=$?
cat "$work/console.log"
if ((vm_status != 0)); then
    printf 'Fedora VM exited with status %s\n' "$vm_status" >&2
    exit "$vm_status"
fi
if ! grep -aFq 'SELinux development gateway checks passed' "$work/console.log"; then
    echo 'Fedora did not report a successful gateway check' >&2
    exit 1
fi
