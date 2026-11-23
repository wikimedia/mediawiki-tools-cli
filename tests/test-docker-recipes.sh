#!/usr/bin/env bash
# Apply every embedded recipe on a disposable Docker host.
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/functions.sh"
MW_BIN=$(test_cli_binary "${MW_BIN:-}")
export MW_BIN MWCLI_CONTEXT_TEST=1
source "$SCRIPT_DIR/pretest-mediawiki.sh"
set -euo pipefail

MWCLI_TEST_RECIPES=1 go test -v -count=1 -timeout=75m ./tests -run '^TestEmbeddedRecipes$'
timeout 15m bash "$SCRIPT_DIR/test-docker-storage.sh"
