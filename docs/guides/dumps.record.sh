#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

# Live requests: two samples and one resumed 21 MiB download, about 65 MiB total.
GOFLAGS=-buildvcs=false make build BINARY_NAME=mw
record_work=$(mktemp -d)
trap 'rm -rf -- "$record_work"' EXIT
mkdir -p "$record_work/bin" _recordings
export MWCLI_RECORD_BASH MWCLI_RECORD_WORK MWCLI_RECORD_BIN
MWCLI_RECORD_BASH=$(command -v bash)
MWCLI_RECORD_WORK=$record_work
MWCLI_RECORD_BIN="$repo_root/bin/mw"
cat > "$record_work/bin/bash" <<'SHELL'
#!/bin/sh
unset BASH_ENV ENV PROMPT_COMMAND
export PS1='> '
cd "$MWCLI_RECORD_WORK"
exec "$MWCLI_RECORD_BASH" --noprofile --norc "$@"
SHELL
ln -s "$MWCLI_RECORD_BIN" "$record_work/bin/mw"
chmod +x "$record_work/bin/bash"
export PATH="$record_work/bin:$PATH"

"${MISE:-mise}" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/dumps.tape"
test -e "$record_work/interrupted"
test -s "$record_work/wikidata-iwlinks.sql.gz"
test ! -e "$record_work/wikidata-iwlinks.sql.gz.part"
gzip -t "$record_work/wikidata-iwlinks.sql.gz"
python3 tools/clean-recording-text.py _recordings/dumps.txt
ffmpeg -hide_banner -loglevel error -y -i _recordings/dumps.png -frames:v 1 _recordings/dumps.webp
