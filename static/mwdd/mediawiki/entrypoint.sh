#!/bin/bash

# MediaWiki / www-data needs to be able to write here to create dirs etc for different sites
chmod 777 /var/www/html/w/images/docker

# Then execute the regular entrypoint
# https://gerrit.wikimedia.org/r/plugins/gitiles/releng/dev-images/+/refs/heads/master/dockerfiles/stretch-php72-fpm-apache2-xdebug/Dockerfile.template#38
/entrypoint.sh $@
