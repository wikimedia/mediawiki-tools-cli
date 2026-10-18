#!/usr/bin/env bash
# Native Windows Docker endpoint and self-update checks.
set -euo pipefail

if [[ "${RUNNER_OS:-}" != "Windows" ]]; then
    echo "This test requires a native Windows runner"
    exit 1
fi
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bash "$SCRIPT_DIR/check-platform.sh"
