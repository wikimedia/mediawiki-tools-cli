#!/usr/bin/env bash
# Tests in this file do not require an internet connection

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh

export MWCLI_CONTEXT_TEST=1

test_command_success ./bin/mw version

test_command_success ./bin/mw debug
test_command_success ./bin/mw debug events
test_command_success ./bin/mw debug events cat
test_command_success ./bin/mw debug events submit

test_command_success ./bin/mw config
test_command_success ./bin/mw config show
test_command_success ./bin/mw config where
# Roundtrip setting a single value
PREVIOUS_TELEMETRY_VAL=$(./bin/mw config get telemetry)
test_command_success ./bin/mw config set telemetry foo
test_command ./bin/mw config get telemetry "foo"
./bin/mw config set telemetry "$PREVIOUS_TELEMETRY_VAL"

# Help topics...
test_command_success ./bin/mw output

# gitlab: Test command is registered and generally works
test_command_success ./bin/mw gitlab
test_command_success ./bin/mw gitlab alias list

# cloudvps ssh: exercise command parsing and exit status without keys or network.
SSH_TEST_DIR=$(mktemp -d)
cat > "$SSH_TEST_DIR/ssh" <<'SSH_FIXTURE'
#!/bin/sh
for arg in "$@"; do
    if [ "$arg" = "-G" ]; then
        printf '%s\n' 'user developer' 'proxyjump none' 'connecttimeout none'
        exit 0
    fi
done
printf '%s\n' "$@"
exit "${MWCLI_TEST_SSH_EXIT_STATUS:-0}"
SSH_FIXTURE
chmod +x "$SSH_TEST_DIR/ssh"
test_command env PATH="$SSH_TEST_DIR:$PATH" ./bin/mw --no-interaction cloudvps ssh vm --project project --user developer --region eqiad1 --dry-run "developer@bastion.wmcloud.org"
test_command env PATH="$SSH_TEST_DIR:$PATH" ./bin/mw --no-interaction cloudvps connect --project project --name vm --user developer --region eqiad1 --dry-run "vm.project.eqiad1.wikimedia.cloud"
test_command env PATH="$SSH_TEST_DIR:$PATH" ./bin/mw --no-interaction cloudvps ssh vm --project project --user developer --region eqiad1 -- printf '%s' 'two words' "'printf' '%s' 'two words'"
if env PATH="$SSH_TEST_DIR:$PATH" MWCLI_TEST_SSH_EXIT_STATUS=42 ./bin/mw --no-interaction cloudvps ssh vm --project project --user developer --region eqiad1 -- false; then
    echo "FAIL: cloudvps ssh did not propagate the remote exit status"
    exit 1
else
    SSH_TEST_STATUS=$?
    if [ "$SSH_TEST_STATUS" -ne 42 ]; then
        echo "FAIL: cloudvps ssh returned $SSH_TEST_STATUS, expected 42"
        exit 1
    fi
fi
rm -rf "$SSH_TEST_DIR"

# update: Test with local file copy (actually exercises file replacement logic)
# Create a temporary directory for test binary
TEST_BINARY_DIR=$(mktemp -d)
TEST_BINARY_PATH="$TEST_BINARY_DIR/mw"

# Copy the current binary to temp location
cp ./bin/mw "$TEST_BINARY_PATH"

# Test updating with local file path (non-interactive)
test_command_success ./bin/mw update -vv --version "$TEST_BINARY_PATH" --no-interaction

# Cleanup (skip in CI since container is ephemeral)
if [ -z "$CI" ] && [ -z "$GITLAB_CI" ]; then
	rm -rf "$TEST_BINARY_DIR"
fi

# Source updates must work offline without invoking Docker.
bash "$SCRIPT_DIR/test-mediawiki-update-code.sh" "$PWD/bin/mw"
