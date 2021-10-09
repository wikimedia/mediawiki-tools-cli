#!/usr/bin/env bash

set -e # Fail on errors
set -x # Output commands

# Only re fetch MediaWiki if we don't already have it in the cache for this job
if [[ ! -f mediawiki/.gitlab-ci.cache.20210809-04 ]]; then
  rm -rf mediawiki
  apk add --no-cache curl tar
  curl https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/core/+archive/refs/heads/master.tar.gz -o mediawiki.tar.gz
  curl https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/skins/Vector/+archive/refs/heads/master.tar.gz -o vector.tar.gz
  curl https://gerrit.wikimedia.org/r/plugins/gitiles/mediawiki/vendor/+archive/refs/heads/master.tar.gz -o vendor.tar.gz
  mkdir mediawiki
  tar -xf mediawiki.tar.gz -C mediawiki
  mkdir mediawiki/skins/Vector
  tar -xf vector.tar.gz -C mediawiki/skins/Vector
  mkdir mediawiki/vendor
  tar -xf vendor.tar.gz -C mediawiki/vendor
  rm -r *.tar.gz

  # composer install
  docker run --rm -v $PWD/mediawiki:/app -w /app --entrypoint=composer docker-registry.wikimedia.org/dev/stretch-php73-fpm:3.0.0 install --ignore-platform-reqs --no-interaction

  # npm install
  apk add --no-cache npm
  npm --prefix mediawiki ci

  touch mediawiki/.gitlab-ci.cache.20210809-04
fi

# composer update (even when cached) to ensure deps are as up to date as possible
docker run --rm -v $PWD/mediawiki:/app -w /app --entrypoint=composer docker-registry.wikimedia.org/dev/stretch-php73-fpm:3.0.0 update --no-interaction --no-progress --ignore-platform-reqs

# Always remove files that may have been left behind by previous tests and may get in the way?
rm -f mediawiki/LocalSettings.php
