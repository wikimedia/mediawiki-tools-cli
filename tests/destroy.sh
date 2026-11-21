#!/usr/bin/env bash
#
# Easily destroys the mwcli docker test environment

source "$(dirname -- "${BASH_SOURCE[0]}")/functions.sh"
MW_BIN=$(test_cli_binary "${MW_BIN:-}") || exit 1
MWCLI_CONTEXT_TEST=1 "$MW_BIN" dev destroy
