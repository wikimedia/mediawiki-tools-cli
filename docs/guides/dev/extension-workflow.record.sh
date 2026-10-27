#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

context=${CONTEXT:?Set CONTEXT to an initialized disposable wiki checkout without Cite}
GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
export MWCLI_RECORD_BIN="$repo_root/bin/mw"
export MWCLI_RECORD_CONTEXT="$context"
export PATH="$repo_root/bin:$PATH"
code_dir=$(./bin/mw --no-interaction dev --context "$context" mediawiki where)
if [[ -e "$code_dir/extensions/Cite" ]] || grep -q "wfLoadExtension( 'Cite' );" "$code_dir/LocalSettings.php"; then
    echo "Cite is already present in $code_dir; use a clean disposable checkout to record this workflow." >&2
    exit 1
fi
mkdir -p _recordings

mise_bin=${MISE:-mise}
MISE_DATA_DIR=${MISE_DATA_DIR:-$HOME/.local/share/mise} "$mise_bin" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/extension-workflow.tape"

python3 tools/clean-recording-text.py _recordings/dev-extension-workflow.txt
