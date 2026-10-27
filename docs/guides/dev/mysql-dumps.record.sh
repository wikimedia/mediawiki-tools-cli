#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

context=${CONTEXT:-recording}
GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
export PATH="$repo_root/bin:$PATH"
export MWCLI_RECORD_BIN="$repo_root/bin/mw"
export MWCLI_RECORD_CONTEXT="$context"

mw() { "$MWCLI_RECORD_BIN" --no-interaction "$@"; }
if ! docker ps --filter "label=com.docker.compose.project=mwcli-mwdd-$context" --filter status=running --format '{{.Names}}' | rg -q 'mysql-1$'; then
    echo "MySQL is not running in context '$context'. Start a local wiki and database first." >&2
    exit 1
fi

mkdir -p _recordings
dump_file=_recordings/mw-default.sql
if [[ -e $dump_file ]]; then
    echo "Refusing to overwrite $dump_file; move or remove it first." >&2
    exit 1
fi
cleanup() {
    rm -f "$dump_file"
    mw dev --context "$context" compose exec -- -T mysql mariadb -uroot -ptoor -e 'DROP DATABASE IF EXISTS recording_restore' >/dev/null 2>&1 || true
    mw dev --context "$context" compose exec -- -T mysql rm -f /tmp/mw-default.sql >/dev/null 2>&1 || true
}
trap cleanup EXIT

"${MISE:-mise}" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/mysql-dumps.tape"

python3 tools/clean-recording-text.py _recordings/dev-mysql-dump.txt
