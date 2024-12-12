FROM "${SHELLBOX_MEDIA_IMAGE:-docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox:2024-12-07-073046-media81}"

USER root
RUN echo 'env[SHELLBOX_SECRET_KEY] = ${SHELLBOX_SECRET_KEY}' >> /etc/php/7.4/fpm/pool.d/FCGI_UNIX.conf

USER www-data
