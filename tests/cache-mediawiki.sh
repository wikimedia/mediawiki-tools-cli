#!/usr/bin/env bash
set -e

# Weekly rotating cache key, to speed up CI, but not get too out of date...
CURRENT_YEAR=$(date +%G)
CURRENT_WEEK=$(date +%V)
CACHE_KEY_DATE="$CURRENT_YEAR-$CURRENT_WEEK"
MEDIAWIKI_CACHE_REFRESHED=0

# Only re fetch MediaWiki if we don't already have it in the cache for this job
if [[ ! -f .mediawiki/.mwcli.ci.cache.$CACHE_KEY_DATE ]]; then
  # Delete any previous cache
  rm -rf .mediawiki

  mkdir .mediawiki

  download_archive() {
    local gerrit_url=$1
    local github_url=$2
    local output_file=$3
    ARCHIVE_STRIP_COMPONENTS=0

    if curl --fail --location --silent --show-error --retry 3 --retry-all-errors --retry-delay 2 "$gerrit_url" -o "$output_file"; then
      return 0
    fi

    echo "Gerrit archive unavailable; falling back to the Wikimedia GitHub mirror"
    rm -f "$output_file"
    ARCHIVE_STRIP_COMPONENTS=1
    curl --fail --location --silent --show-error --retry 3 --retry-all-errors --retry-delay 2 "$github_url" -o "$output_file"
  }

  download_archive \
    https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/core/+archive/refs/heads/master.tar.gz \
    https://codeload.github.com/wikimedia/mediawiki/tar.gz/refs/heads/master \
    mediawiki.tar.gz
  tar -xf mediawiki.tar.gz --strip-components="$ARCHIVE_STRIP_COMPONENTS" -C .mediawiki
  rm mediawiki.tar.gz

  mkdir -p .mediawiki/skins/Vector
  download_archive \
    https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/skins/Vector/+archive/refs/heads/master.tar.gz \
    https://codeload.github.com/wikimedia/mediawiki-skins-Vector/tar.gz/refs/heads/master \
    vector.tar.gz
  tar -xf vector.tar.gz --strip-components="$ARCHIVE_STRIP_COMPONENTS" -C .mediawiki/skins/Vector
  rm vector.tar.gz

  mkdir -p .mediawiki/vendor
  download_archive \
    https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/vendor/+archive/refs/heads/master.tar.gz \
    https://codeload.github.com/wikimedia/mediawiki-vendor/tar.gz/refs/heads/master \
    vendor.tar.gz
  tar -xf vendor.tar.gz --strip-components="$ARCHIVE_STRIP_COMPONENTS" -C .mediawiki/vendor
  rm vendor.tar.gz

  if [[ "${MWCLI_CI_NATIVE_DEPS:-0}" = 1 ]]; then
    # Shared CI runners provide these tools in the preparation image.
    (
      cd .mediawiki
      composer install --ignore-platform-reqs --no-interaction --no-progress
      npm ci
    )
  else
    # Preserve Docker-based preparation for local integration test runs.
    docker run -u "$(id -u):$(id -g)" --rm -v "$PWD/.mediawiki:/app" -w /app --entrypoint=composer docker-registry.wikimedia.org/dev/buster-php74-fpm:1.0.0-s3 install --ignore-platform-reqs --no-interaction --no-progress
    docker run -u "$(id -u):$(id -g)" --rm -v "$PWD/.mediawiki:/app" -w /app --entrypoint=npm docker-registry.wikimedia.org/releng/node14-test-browser:0.0.2-s3 ci
  fi

  touch .mediawiki/.mwcli.ci.cache.$CACHE_KEY_DATE
  MEDIAWIKI_CACHE_REFRESHED=1
fi
