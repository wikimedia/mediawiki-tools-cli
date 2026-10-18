#!/usr/bin/env bash
# T403412: exercise the installed CLI through Docker's saved context and overrides.
set -euo pipefail

echo 'Checking controlled Docker API fixture (no Docker engine)'

root="$(mktemp -d)"
fixture_pid=""
cleanup() {
    if [[ -n "$fixture_pid" ]]; then
        kill "$fixture_pid" 2>/dev/null || true
        wait "$fixture_pid" 2>/dev/null || true
    fi
    rm -rf "$root"
}
trap cleanup EXIT
python3 tests/fixtures/docker-context.py "$root" >"$root/fixture.log" 2>&1 &
fixture_pid=$!
for ((attempt=0; attempt<100; attempt++)); do
    [[ -f "$root/ready" ]] && break
    if ! kill -0 "$fixture_pid" 2>/dev/null; then
        cat "$root/fixture.log"
        exit 1
    fi
    sleep 0.1
done
[[ -f "$root/ready" ]] || { cat "$root/fixture.log"; echo 'Docker fixture did not start'; exit 1; }

# Paths written by Python are native paths, including when Bash runs on Windows.
export DOCKER_CONFIG="$(cat "$root/docker-config")"
export XDG_CONFIG_HOME="$(cat "$root/config-home")"
export APPDATA="$XDG_CONFIG_HOME" LOCALAPPDATA="$XDG_CONFIG_HOME"
export MWCLI_CONTEXT_TEST=1
unset GITLAB_CI CI_PROJECT_DIR DOCKER_HOST DOCKER_CONTEXT DOCKER_API_VERSION
unset DOCKER_TLS DOCKER_TLS_VERIFY DOCKER_CERT_PATH
unset MWCLI_MEDIAWIKI_ENV_COMMAND MW_DOCKER_MEDIAWIKI_GET_CODE
# Stub only the Compose availability prerequisite. Docker API traffic must go
# through the real CLI to the fixture; no containers or Compose commands run.
export PATH="$root/bin:$PATH"
binary="${1:-./bin/mw}"
if [[ $# == 0 && -f ./bin/mw.exe ]]; then
    binary=./bin/mw.exe
fi
host="$(cat "$root/host")"
expected="$(cat "$root/expected")"

check_context() {
    local label="$1" before after output
    before="$(wc -l < "$root/requests")"
    echo "Checking Docker context: $label"
    if ! output="$("$binary" --no-interaction dev --context=context-regression where 2>&1)"; then
        echo "$output"
        exit 1
    fi
    echo "$output"
    [[ "$output" == *"$expected"* ]] || { echo 'FAIL: CLI did not finish dev where'; exit 1; }
    after="$(wc -l < "$root/requests")"
    ((after > before)) || { echo 'FAIL: CLI did not ping the selected endpoint'; exit 1; }
    echo "PASS: $label ($host)"
}

check_context 'saved context without environment overrides'
printf '{"currentContext":"missing-saved-context"}\n' > "$root/docker/config.json"
export DOCKER_CONTEXT=desktop-linux
check_context 'DOCKER_CONTEXT overrides saved context'
export DOCKER_CONTEXT=missing-explicit-context DOCKER_HOST="$host"
check_context 'DOCKER_HOST overrides missing context'

unset DOCKER_HOST DOCKER_CONTEXT
printf '{"currentContext":"desktop-linux"}\n' > "$root/docker/config.json"
kill "$fixture_pid"
wait "$fixture_pid" 2>/dev/null || true
fixture_pid=""
echo 'Checking Docker context: unavailable endpoint diagnostic'
if output="$("$binary" --no-interaction dev --context=context-regression where 2>&1)"; then
    echo 'FAIL: CLI accepted an unavailable Docker endpoint'
    exit 1
fi
echo "$output"
[[ "$output" == *"Docker is unavailable:"* && "$output" == *"$host"* ]] || {
    echo 'FAIL: Docker failure did not identify the selected endpoint'
    exit 1
}
echo 'PASS: unavailable endpoint fails with its socket or named pipe in the diagnostic'
