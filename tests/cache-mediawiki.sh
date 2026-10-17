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
  curl --fail --location --retry 3 https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/core/+archive/refs/heads/master.tar.gz -o mediawiki.tar.gz
  tar -xf mediawiki.tar.gz -C .mediawiki
  rm mediawiki.tar.gz

  mkdir .mediawiki/skins/Vector
  curl --fail --location --retry 3 https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/skins/Vector/+archive/refs/heads/master.tar.gz -o vector.tar.gz
  tar -xf vector.tar.gz -C .mediawiki/skins/Vector
  rm vector.tar.gz

  mkdir .mediawiki/vendor
  curl --fail --location --retry 3 https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/vendor/+archive/refs/heads/master.tar.gz -o vendor.tar.gz
  tar -xf vendor.tar.gz -C .mediawiki/vendor
  rm vendor.tar.gz

  # composer install (for update and dev deps)
  # TODO use on disk cache
  docker run -u "$(id -u):$(id -g)" --rm -v "$PWD/.mediawiki:/app" -w /app --entrypoint=composer docker-registry.wikimedia.org/dev/buster-php74-fpm:1.0.0-s3 install --ignore-platform-reqs --no-interaction --no-progress

  # npm install
  # TODO use on disk cache
  docker run -u "$(id -u):$(id -g)" --rm -v "$PWD/.mediawiki:/app" -w /app --entrypoint=npm docker-registry.wikimedia.org/releng/node14-test-browser:0.0.2-s3 ci

  touch .mediawiki/.mwcli.ci.cache.$CACHE_KEY_DATE
  MEDIAWIKI_CACHE_REFRESHED=1
fi
