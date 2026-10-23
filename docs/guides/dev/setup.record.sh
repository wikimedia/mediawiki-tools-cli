#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

context=${CONTEXT:-recording-setup}
code_dir=${MEDIAWIKI_CODE:-}
if [[ -z $code_dir || ! -f $code_dir/LocalSettings.php || ! -d $code_dir/skins/Vector ]]; then
    echo 'Set MEDIAWIKI_CODE to a MediaWiki checkout with Vector and LocalSettings.php.' >&2
    exit 1
fi
if [[ ! $context =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
    echo 'CONTEXT must use lowercase letters, numbers, hyphens or underscores.' >&2
    exit 1
fi

project="mwcli-mwdd-$context"
if [[ -n $(docker ps -aq --filter "label=com.docker.compose.project=$project") || \
      -n $(docker volume ls -q --filter "label=com.docker.compose.project=$project") ]]; then
    echo "Context '$context' already has Docker state; use a new context for this setup recording." >&2
    exit 1
fi

GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
export PATH="$repo_root/bin:$PATH"
export MWCLI_RECORD_BIN="$repo_root/bin/mw"
export MWCLI_RECORD_CONTEXT="$context"

mw() { "$MWCLI_RECORD_BIN" --no-interaction "$@"; }
mw dev --context "$context" env set MEDIAWIKI_VOLUMES_CODE "$code_dir"

mkdir -p _recordings
"${MISE:-mise}" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/setup.tape"
