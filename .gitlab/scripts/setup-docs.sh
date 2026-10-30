#!/usr/bin/env bash
# Source this so the pinned documentation tools stay on the job's PATH.
set -euo pipefail
if ! command -v python3 >/dev/null; then
  apt-get update
  apt-get install -y --no-install-recommends python3
fi
curl -fsSL https://mise.run -o /tmp/install-mise.sh
MISE_VERSION=2026.10.12 sh /tmp/install-mise.sh
export PATH="$HOME/.local/bin:$PATH"
mise trust mise.toml
mise install
