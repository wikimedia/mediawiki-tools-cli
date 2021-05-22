#!/bin/bash

# MediaWiki / www-data needs to be able to write here to create dirs etc for different sites
# TODO do this some other way to avoid needing to override the entrypoint...
chmod 777 /var/www/html/w/images/docker

# Then execute the regular entrypoint
# https://gerrit.wikimedia.org/r/plugins/gitiles/releng/dev-images/+/refs/heads/master/dockerfiles/stretch-php72-fpm/Dockerfile.template#32
/php_entrypoint.sh $@
