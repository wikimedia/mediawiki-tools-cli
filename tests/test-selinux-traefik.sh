#!/usr/bin/env bash
# Exercise the development gateway and a MediaWiki installation on enforcing Fedora.
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

# Pass the tested CLI, Compose client and production images into the VM.
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

install -m 0644 "${MWCLI_SMOKE_SNAPSHOT:?provide the MediaWiki CI snapshot archive}" "$work/mediawiki.tar.gz"

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
test "$(podman info --format '{{.Host.Security.SELinuxEnabled}}')" = true
if [[ "$mode" == rootless ]]; then
    uid=$(id -u fedora)
    systemctl start "user@$uid.service"
    runtime="/run/user/$uid"
    as_fedora=(runuser -u fedora -- env "XDG_RUNTIME_DIR=$runtime" "DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime/bus")
    "${as_fedora[@]}" systemctl --user start podman.socket
    "${as_fedora[@]}" podman info --format '{{.Host.Security.Rootless}}' | grep -x true
    "${as_fedora[@]}" /usr/local/bin/mw --no-interaction update --dry-run
    echo 'PASS: checked for a CLI update'
    "${as_fedora[@]}" podman load --input /run/mwcli-ci/images.tar
    "${as_fedora[@]}" "DOCKER_HOST=unix://$runtime/podman/podman.sock" \
        "MWCLI_SMOKE_DOCKER_SOCKET=$runtime/podman/podman.sock" \
        bash /run/mwcli-ci/stack.sh /usr/local/bin/mw /run/mwcli-ci/mediawiki.tar.gz
else
    systemctl start podman.socket
    ln -s /run/podman/podman.sock /run/docker.sock
    podman info --format '{{.Host.Security.Rootless}}' | grep -x false
    HOME=/root mw --no-interaction update --dry-run
    echo 'PASS: checked for a CLI update'
    podman load --input /run/mwcli-ci/images.tar
    HOME=/root DOCKER_HOST=unix:///run/podman/podman.sock \
        bash /run/mwcli-ci/stack.sh /usr/local/bin/mw /run/mwcli-ci/mediawiki.tar.gz
fi
GUEST
} > "$work/user-data"
printf 'instance-id: mwcli-selinux-ci\n' > "$work/meta-data"
cp tests/test-platform-stack.sh "$work/stack.sh"
genisoimage -quiet -output "$work/seed.iso" -volid CIDATA -joliet -rock "$work/user-data" "$work/meta-data" "$work/stack.sh" "$work/mediawiki.tar.gz" "$work/images.tar" "$work/cli" "$work/docker" "$work/docker-compose"

# Use KVM when offered by the runner, with software emulation as a fallback.
vm_status=0
timeout --kill-after=30s 20m qemu-system-x86_64 \
    -machine accel=kvm:tcg -m 3072 -smp 2 -nographic -no-reboot \
    -drive "file=$work/guest.qcow2,if=virtio,format=qcow2" \
    -drive "file=$work/seed.iso,media=cdrom,readonly=on" \
    -nic user,model=virtio-net-pci > "$work/console.log" 2>&1 || vm_status=$?
cat "$work/console.log"
if ((vm_status != 0)); then
    printf 'Fedora VM exited with status %s\n' "$vm_status" >&2
    exit "$vm_status"
fi
if ! grep -aFq 'Gateway and MediaWiki smoke checks passed' "$work/console.log"; then
    echo 'Fedora did not report a successful gateway check' >&2
    exit 1
fi
