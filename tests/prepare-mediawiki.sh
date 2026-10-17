#!/usr/bin/env bash
# Prepare a clean, weekly refreshed snapshot for the Docker integration jobs.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ARCHIVE=.cache/mediawiki.tar.gz
mkdir -p .cache

if [[ -f "$ARCHIVE" ]]; then
  tar -xzf "$ARCHIVE"
fi

source "$SCRIPT_DIR/cache-mediawiki.sh"

# Reuse a warm archive without compressing the same tree again.
if [[ ! -f "$ARCHIVE" || "$MEDIAWIKI_CACHE_REFRESHED" = 1 ]]; then
  tar -cf - .mediawiki | gzip -1 > "$ARCHIVE"
fi
