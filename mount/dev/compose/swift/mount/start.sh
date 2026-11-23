#!/bin/bash
# Derived from wmf-claude 003fa611cd670d030a0063986a876960f2ba7214 (MIT; LICENSE).
set -euo pipefail
: "${SWIFT_TEMP_URL_KEY:?Set SWIFT_TEMP_URL_KEY}"
python3 /usr/local/bin/configure.py
rm -f /var/run/swift-ready
mkdir -p /srv/node/d1 /var/cache/swift /var/run/swift
chown swift:swift /srv/node/d1 /var/cache/swift /var/run/swift
pids=()
cleanup() {
    rm -f /var/run/swift-ready
    kill "${pids[@]}" 2>/dev/null || true
    wait || true
}
trap cleanup EXIT
trap 'exit 0' TERM INT
memcached -u memcache -l 127.0.0.1 &
pids+=("$!")
for server in account container object; do
    "swift-$server-server" "/etc/swift/$server-server.conf" --verbose &
    pids+=("$!")
done
swift-proxy-server /etc/swift/proxy-server.conf --verbose &
pids+=("$!")
ready=false
for ((attempt = 0; attempt < 20; attempt++)); do
    if timeout 3 swift -A http://127.0.0.1:8080/auth/v1.0 -U mw:media -K "$SWIFT_MEDIA_KEY" \
        post -m "Temp-URL-Key:$SWIFT_TEMP_URL_KEY" >/dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 1
done
if [[ $ready != true ]]; then
    echo 'Swift account initialization failed' >&2
    exit 1
fi
touch /var/run/swift-ready
echo 'Swift account AUTH_mw is ready'
# Stop the whole container if any server exits; restarting reinitializes readiness.
wait -n "${pids[@]}"
exit 1
