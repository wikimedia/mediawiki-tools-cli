#!/usr/bin/env bash
# Exercise the Docker Compose CLI and Docker API against rootless Podman.
set -euo pipefail

mw=$(realpath "${MW_BIN:-bin/mw}")
dev() { "$mw" --no-interaction dev --context podman-ci "$@"; }
source_dir=$(mktemp -d)
cleanup() {
    dev compose down -- --volumes
    rm -rf -- "$source_dir"
}
trap cleanup EXIT

dev env set PORT 6197
dev env set MEDIAWIKI_VOLUMES_CODE "$source_dir"
custom=$(dev custom --name custom-podman where)
cat >"$custom" <<'COMPOSE'
services:
  custom-podman:
    image: docker.io/library/alpine:3.23
    command: ["sleep", "infinity"]
    volumes:
      - ${MEDIAWIKI_VOLUMES_CODE}:/workspace
COMPOSE

dev compose up -- -d custom-podman
dev compose ps | grep -F custom-podman
dev custom --name custom-podman exec -- id -u | tr -d '\r' | grep -Fx 0
dev custom --name custom-podman exec -- touch /workspace/test-file
test -O "$source_dir/test-file"
dev compose stop -- custom-podman
dev compose start -- custom-podman
dev custom --name custom-podman exec --user nobody -- id -u | tr -d '\r' | grep -Fx 65534

dev traefik create
curl --fail --silent --show-error --retry 20 --retry-all-errors --retry-delay 2 --max-time 5 \
    http://127.0.0.1:6197/proxy/dashboard/ -o "$source_dir/dashboard.html"
grep -iq dashboard "$source_dir/dashboard.html"
