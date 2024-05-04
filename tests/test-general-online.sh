#!/usr/bin/env bash
# Tests in this file require an internet connection

set -e # Fail on errors
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source $SCRIPT_DIR/functions.sh

export MWCLI_CONTEXT_TEST=1

test_command_success ./bin/mw quip

test_command_success ./bin/mw codesearch search addshore

test_command_success ./bin/mw toolhub tools list
test_command_success ./bin/mw toolhub tools search addshore
test_command_success ./bin/mw toolhub tools get bash

# Ignore linting e2e suggestions for some commands that are too involved with online things for now
# SUGGEST cmd: mw tools: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw tools cp: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw tools exec: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw update: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw wiki: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw wiki page: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw wiki page delete: (end-to-end-test) End to end tests are suggested, none found
# SUGGEST cmd: mw wiki page put: (end-to-end-test) End to end tests are suggested, none found