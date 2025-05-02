FROM "${SHELLBOX_PHP_RPC_IMAGE:-docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2025-04-16-162025-php-rpc81}"

USER root
RUN echo 'env[SHELLBOX_SECRET_KEY] = ${SHELLBOX_SECRET_KEY}' >> /etc/php/8.1/fpm/pool.d/FCGI_UNIX.conf

USER www-data
