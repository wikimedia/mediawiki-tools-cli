#!/usr/bin/env bash

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh

export MWCLI_CONTEXT_TEST=1

# gitlab: Test command is registered and generally works
test_command_success "./bin/mw gitlab"
test_command_success "./bin/mw gitlab alias list"