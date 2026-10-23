#!/bin/sh
# Preserve the invoking user's identity and Quibble's arguments. Memcached
# requires an explicit user when Quibble runs as root (e.g. in CI).
if [ "$(id -u)" = 0 ]; then
    exec /usr/bin/memcached --user=root "$@"
fi
exec /usr/bin/memcached "$@"
