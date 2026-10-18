#!/usr/bin/env bash
# Validate real Windows executable replacement and config access after updating.
set -euo pipefail

if [[ "${RUNNER_OS:-}" != "Windows" ]]; then
    echo "This test requires a native Windows runner"
    exit 1
fi
export MWCLI_CONTEXT_TEST=1

TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT
mkdir -p "$TEST_ROOT/installed cli" "$TEST_ROOT/update artifact" "$TEST_ROOT/config home"
export APPDATA="$(cygpath -w "$TEST_ROOT/config home")"
INSTALLED="$TEST_ROOT/installed cli/mw.exe"
REPLACEMENT="$TEST_ROOT/update artifact/mw.exe"
cp ./bin/mw.exe "$INSTALLED"

# Distinct bytes let us prove the delayed replacement actually completed.
go build -ldflags='-X main.GitCommit=windows-update-fixture' -o "$REPLACEMENT" .
EXPECTED_HASH="$(sha256sum "$REPLACEMENT")"
EXPECTED_HASH="${EXPECTED_HASH%% *}"
ORIGINAL_HASH="$(sha256sum "$INSTALLED")"
if [[ "${ORIGINAL_HASH%% *}" == "$EXPECTED_HASH" ]]; then
    echo "FAIL: replacement fixture must differ from the installed binary"
    exit 1
fi

"$INSTALLED" --no-interaction config set gerrit.username before-update
"$INSTALLED" update --version "$(cygpath -m "$REPLACEMENT")" --no-interaction

for attempt in {1..30}; do
    ACTUAL_HASH="$(sha256sum "$INSTALLED" 2>/dev/null)" || ACTUAL_HASH=""
    if [[ "${ACTUAL_HASH%% *}" == "$EXPECTED_HASH" ]]; then
        break
    fi
    sleep 1
done
if [[ "${ACTUAL_HASH%% *}" != "$EXPECTED_HASH" ]]; then
    echo "FAIL: Windows self-update did not install the replacement binary"
    exit 1
fi

# Start the replaced executable and check existing config remains readable/writable.
"$INSTALLED" --no-interaction config show
[[ "$("$INSTALLED" --no-interaction config get gerrit.username)" == before-update ]]
"$INSTALLED" --no-interaction config set gerrit.username after-update
[[ "$("$INSTALLED" --no-interaction config get gerrit.username)" == after-update ]]
echo "PASS: updated Windows binary starts and preserves writable user config"
