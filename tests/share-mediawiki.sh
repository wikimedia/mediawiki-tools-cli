#!/usr/bin/env bash
# Publish once per ISO week and preparation recipe; hand over only a reference.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .cache
: "${CI_API_V4_URL:?}" "${CI_PROJECT_ID:?}" "${CI_JOB_TOKEN:?}" "${CI_JOB_IMAGE:?}"
recipe=$( { sha256sum tests/cache-mediawiki.sh tests/prepare-mediawiki.sh tests/share-mediawiki.sh; printf '%s\n' "$CI_JOB_IMAGE"; } | sha256sum | cut -d' ' -f1)
version="$(date -u +%G-%V)-$recipe"
base="/projects/$CI_PROJECT_ID/packages/generic/mediawiki-ci-snapshot/$version"
manifest=.cache/mediawiki-snapshot.txt
headers=(--header "JOB-TOKEN: $CI_JOB_TOKEN" --user-agent 'a-wikimedia-cli-ci (https://a-wikimedia-cli.toolforge.org/)')

# Only HTTP 404 is a cache miss.
fetch() {
  local status
  status=$(curl --location --silent --show-error --retry 3 --retry-all-errors \
    --user-agent 'a-wikimedia-cli-ci (https://a-wikimedia-cli.toolforge.org/)' --output "$2" --write-out '%{http_code}' "$CI_API_V4_URL$1")
  case "$status" in
    200) return 0 ;;
    404) return 1 ;;
    *) echo "Snapshot registry returned HTTP $status" >&2; exit 1 ;;
  esac
}

if fetch "$base/snapshot.txt" "$manifest"; then
  read -r path checksum extra < "$manifest"
  if [[ "$checksum" =~ ^[0-9a-f]{64}$ && "$path" = "$base/$checksum.tar.gz" && -z "$extra" ]]; then
    echo "Reusing shared MediaWiki snapshot $version"
    exit 0
  fi
  echo "Invalid shared MediaWiki snapshot manifest" >&2
  exit 1
fi

# A recipe change or a new week requires a genuinely fresh dependency tree.
rm -rf .mediawiki .cache/mediawiki.tar.gz
bash tests/prepare-mediawiki.sh
checksum=$(sha256sum .cache/mediawiki.tar.gz | cut -d' ' -f1)
path="$base/$checksum.tar.gz"
curl --fail --silent --show-error --retry 3 --retry-all-errors \
  "${headers[@]}" --upload-file .cache/mediawiki.tar.gz "$CI_API_V4_URL$path"
printf '%s %s\n' "$path" "$checksum" > "$manifest"
# Publish the pointer last so readers never see an incomplete upload.
curl --fail --silent --show-error --retry 3 --retry-all-errors \
  "${headers[@]}" --upload-file "$manifest" "$CI_API_V4_URL$base/snapshot.txt"

rm -f .cache/mediawiki.tar.gz
