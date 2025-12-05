FROM "${SHELLBOX_PHP_RPC_IMAGE:-docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2025-11-26-175023-php-rpc83}"

USER root
RUN PHP_VERSION=$(ls /etc/php/ | sort -V | tail -1) && \
    mkdir -p /etc/php/${PHP_VERSION}/fpm/pool.d && \
    echo 'env[SHELLBOX_SECRET_KEY] = ${SHELLBOX_SECRET_KEY}' >> /etc/php/${PHP_VERSION}/fpm/pool.d/FCGI_UNIX.conf

USER www-data
