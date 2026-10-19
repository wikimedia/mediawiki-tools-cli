#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

context=${CONTEXT:-recording}
image=${MWCLI_RECORD_IMAGE:-ubuntu:24.04}
GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
export PATH="$repo_root/bin:$PATH"
export MWCLI_RECORD_BIN="$repo_root/bin/mw"
export MWCLI_RECORD_IMAGE="$image"
export MWCLI_RECORD_NETWORK=$("$repo_root/bin/mw" --no-interaction dev --context "$context" env get TRAEFIK_DOCKER_NETWORK)

if ! docker network inspect "$MWCLI_RECORD_NETWORK" >/dev/null 2>&1; then
    echo "Docker network '$MWCLI_RECORD_NETWORK' is not running; start the MediaWiki context first." >&2
    exit 1
fi

mkdir -p _recordings
"${MISE:-mise}" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/api.tape"
