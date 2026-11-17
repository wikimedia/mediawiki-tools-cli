#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../../.." && pwd)
cd "$repo_root"

# Replay the supplied terminal output; never invoke mw or connect to Toolforge.
record_bin=$(mktemp -d)
trap 'rm -rf -- "$record_bin"' EXIT
export MWCLI_RECORD_BASH
MWCLI_RECORD_BASH=$(command -v bash)
cat > "$record_bin/bash" <<'SHELL'
#!/bin/sh
unset BASH_ENV ENV PROMPT_COMMAND
export PS1='> '
exec "$MWCLI_RECORD_BASH" --noprofile --norc "$@"
SHELL
cat > "$record_bin/mw" <<'REPLAY'
#!/bin/sh
sleep 0.7
case "$*" in
    "toolforge replicas sql itwiki -- -e SELECT COUNT(*) FROM user")
        cat <<'OUTPUT'
+----------+
| COUNT(*) |
+----------+
|  3232527 |
+----------+
Connection to login.toolforge.org closed.
OUTPUT
        ;;
    "toolforge toolsdb list --tool wikicrowd")
        printf '%s\n' 's54943__laravel'
        ;;
    "toolforge toolsdb sql s54943__laravel --tool wikicrowd -- -e SHOW tables")
        cat <<'OUTPUT'
+---------------------------+
| Tables_in_s54943__laravel |
+---------------------------+
| answers                   |
| edits                     |
| failed_jobs               |
| job_batches               |
| migrations                |
| personal_access_tokens    |
| question_groups           |
| questions                 |
| skipped_questions         |
| users                     |
+---------------------------+
Connection to login.toolforge.org closed.
OUTPUT
        ;;
    "toolforge toolsdb sql s54943__laravel --tool wikicrowd -- -e SELECT COUNT(*) from answers")
        cat <<'OUTPUT'
+----------+
| COUNT(*) |
+----------+
|  1369493 |
+----------+
Connection to login.toolforge.org closed.
OUTPUT
        ;;
    *)
        printf '%s\n' 'No captured output for this command.' >&2
        exit 1
        ;;
esac
REPLAY
chmod +x "$record_bin/bash" "$record_bin/mw"
export PATH="$record_bin:$PATH"

mkdir -p _recordings
"${MISE:-mise}" exec vhs@0.12.1 ttyd@1.7.7 -- vhs "$script_dir/wiki-replicas.tape"
python3 tools/clean-recording-text.py _recordings/toolforge-sql.txt
