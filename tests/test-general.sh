#!/usr/bin/env bash
# Tests in this file do not require an internet connection

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh

export MWCLI_CONTEXT_TEST=1

test_command_success "./bin/mw version"

test_command_success "./bin/mw debug"
test_command_success "./bin/mw debug events"
test_command_success "./bin/mw debug events cat"
test_command_success "./bin/mw debug events submit"

test_command_success "./bin/mw config"
test_command_success "./bin/mw config show"
test_command_success "./bin/mw config where"
# Roundtrip setting a single value
PREVIOUS_TELEMETRY_VAL=$(./bin/mw config get telemetry)
test_command_success "./bin/mw config set telemetry foo"
test_command "./bin/mw config get telemetry" "foo"
./bin/mw config set telemetry $PREVIOUS_TELEMETRY_VAL

# Help topics...
test_command_success "./bin/mw output"

# gitlab: Test command is registered and generally works
test_command_success "./bin/mw gitlab"
test_command_success "./bin/mw gitlab alias list"