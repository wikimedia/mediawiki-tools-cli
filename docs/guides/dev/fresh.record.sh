#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

context=${CONTEXT:-recording}
spec=${SPEC:-tests/selenium/specs/recentchanges.js}
GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
bin="$repo_root/bin/mw"
code_dir=$("$bin" --no-interaction dev --context "$context" env get MEDIAWIKI_VOLUMES_CODE)
if [[ ! -x $code_dir/node_modules/.bin/wdio ]]; then
    echo "Fresh dependencies are missing; run 'mw dev --context $context mediawiki fresh npm ci' first." >&2
    exit 1
fi

mkdir -p _recordings
started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
"$bin" --no-interaction dev --context "$context" mediawiki fresh record "$spec"
recording=$(find "$code_dir/tests/selenium/log" -type f -name '*.mp4' -newermt "$started" -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2-)
if [[ -z $recording || ! -s $recording ]]; then
    echo 'Fresh completed without producing a new MP4 recording.' >&2
    exit 1
fi
cp -- "$recording" _recordings/dev-fresh-recentchanges.mp4
echo "Saved _recordings/dev-fresh-recentchanges.mp4"
