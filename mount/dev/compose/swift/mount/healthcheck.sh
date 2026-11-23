#!/bin/sh
set -eu
test -f /var/run/swift-ready
exec timeout 5 swift -A http://127.0.0.1:8080/auth/v1.0 -U mw:media -K "$SWIFT_MEDIA_KEY" stat >/dev/null
