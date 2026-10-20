#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

context=${CONTEXT:-recording-setup-final}
GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
export MWCLI_RECORD_BIN="$repo_root/bin/mw"
export MWCLI_RECORD_CONTEXT="$context"
export PATH="$repo_root/bin:$PATH"
mkdir -p _recordings

mise_bin=${MISE:-mise}
MISE_DATA_DIR=${MISE_DATA_DIR:-$HOME/.local/share/mise} "$mise_bin" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/lifecycle.tape"
