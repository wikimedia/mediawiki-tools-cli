FROM "${SHELLBOX_PHP_RPC_IMAGE:-docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2024-06-13-133425-php-rpc}"

USER root
RUN echo 'env[SHELLBOX_SECRET_KEY] = ${SHELLBOX_SECRET_KEY}' >> /etc/php/7.4/fpm/pool.d/FCGI_UNIX.conf

USER www-data
