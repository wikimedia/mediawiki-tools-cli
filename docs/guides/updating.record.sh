#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

demo_dir=${MWCLI_UPDATE_DEMO_DIR:-/tmp/mwcli-update-demo}
mkdir -p "$demo_dir"
GOCACHE=${GOCACHE:-/tmp/mwcli-recording-gocache} GOFLAGS=-buildvcs=false \
  go build -ldflags '-X main.Version=0.32.0' -o "$demo_dir/mw" .
export MWCLI_RECORD_BIN="$demo_dir/mw"
export PATH="$demo_dir:$PATH"
mkdir -p _recordings

mise_bin=${MISE:-mise}
MISE_DATA_DIR=${MISE_DATA_DIR:-$HOME/.local/share/mise} "$mise_bin" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/updating.tape"
