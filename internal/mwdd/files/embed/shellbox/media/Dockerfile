FROM "${SHELLBOX_MEDIA_IMAGE:-docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2023-09-07-142824-media}"

USER root
RUN echo 'env[SHELLBOX_SECRET_KEY] = ${SHELLBOX_SECRET_KEY}' >> /etc/php/7.4/fpm/pool.d/FCGI_UNIX.conf

USER www-data
