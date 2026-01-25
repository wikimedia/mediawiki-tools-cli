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
