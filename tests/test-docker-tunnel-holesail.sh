#!/usr/bin/env bash
# End-to-end network integration test. Requires Docker, curl, Python 3, Linux
# host networking, and public Holesail DHT access. No extra VM.
# CI bootstraps a disposable context; manual runs require an installed default wiki.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
MW_BIN=${MW_BIN:-./bin/mw}
ci_bootstrap=${MWCLI_CI_HOLESAIL_BOOTSTRAP:-${MWCLI_CI_MEDIAWIKI_SNAPSHOT:-0}}
if [ "$ci_bootstrap" = "1" ]; then
    TUNNEL_CONTEXT=${TUNNEL_CONTEXT:-ci-holesail}
else
    TUNNEL_CONTEXT=${TUNNEL_CONTEXT:-default}
fi
CLIENT_PORT=${CLIENT_PORT:-18082}
client_name="mwcli-holesail-proof-$$"
project="mwcli-mwdd-$TUNNEL_CONTEXT"
work=$(mktemp -d)
mw() { "$MW_BIN" --no-interaction docker --context "$TUNNEL_CONTEXT" "$@"; }

for service in cloudflared holesail; do
    if [ -n "$(docker ps -q --filter "label=com.docker.compose.project=$project" --filter "label=com.docker.compose.service=$service")" ]; then
        echo "Stop the existing $service tunnel before this test, or use another context."
        exit 1
    fi
done
previous_provider=$(mw env get TUNNEL_PROVIDER 2>/dev/null || true)
cleanup() {
    docker rm -f "$client_name" >/dev/null 2>&1 || true
    mw tunnel stop --provider holesail >/dev/null 2>&1 || true
    if [ "$ci_bootstrap" = "1" ]; then
        mw destroy >/dev/null 2>&1 || true
        mw env clear >/dev/null 2>&1 || true
    elif [ -n "$previous_provider" ]; then
        mw env set TUNNEL_PROVIDER "$previous_provider" >/dev/null
    else
        mw env delete TUNNEL_PROVIDER >/dev/null
    fi
    rm -rf "$work"
}
trap cleanup EXIT

if [ "$ci_bootstrap" = "1" ]; then
    echo "Preparing an isolated MediaWiki context for the Holesail test."
    source "$SCRIPT_DIR/pretest-mediawiki.sh"
    # The shared preparation script disables errexit for its other callers.
    set -euo pipefail
    mw env clear
    mw env set MEDIAWIKI_VOLUMES_CODE "$(cd "$SCRIPT_DIR/.." && pwd)/.mediawiki"
    mw mediawiki create
    mw mysql create
    mw mediawiki install --dbtype mysql
fi

echo "Starting real Holesail server (connection secret stays in temporary files)."
if ! mw tunnel start --provider holesail >"$work/start.log" 2>&1; then
    # Avoid publishing a connection capability in CI output or a bug report.
    sed -E 's#hs://[a-zA-Z0-9]+#[connection redacted]#g' "$work/start.log"
    exit 1
fi
mw tunnel url > "$work/connection"
mw tunnel status | sed -E 's#hs://[a-zA-Z0-9]+#[connection redacted]#g'
context_dir=$(mw where)
credentials="$context_dir/compose/traefik/auth/credentials"
username=$(sed -n 's/^Username: //p' "$credentials")
password=$(sed -n 's/^Password: //p' "$credentials")
printf 'machine 127.0.0.1 login %s password %s\n' "$username" "$password" > "$work/netrc"
chmod 600 "$work/netrc"

server=$(docker ps -q --filter "label=com.docker.compose.project=$project" --filter label=com.docker.compose.service=holesail)
image=$(docker inspect --format '{{.Image}}' "$server")
# Use host networking for this same-VM client to avoid Docker bridge-to-bridge
# NAT hairpin failures. It never joins the development network. No server ports
# are published: the client still reaches the gateway through Holesail.
docker run -d --name "$client_name" --network host "$image" \
    "$(cat "$work/connection")" --host 127.0.0.1 --port "$CLIENT_PORT" >/dev/null

origin="http://127.0.0.1:$CLIENT_PORT"
status=""
for ((attempt=0; attempt<30; attempt++)); do
    status=$(curl --silent --max-time 5 -o /dev/null -w '%{http_code}' "$origin/proxy/dashboard/" || true)
    if [ "$status" = 401 ]; then break; fi
    sleep 2
done
if [ "$status" != 401 ]; then
    echo "FAIL: tunnel did not reach the authenticated gateway (last status: $status)"
    docker logs "$client_name" 2>&1 | sed -E 's#hs://[a-zA-Z0-9]+#[connection redacted]#g'
    exit 1
fi
echo "PASS: unauthenticated request through the Holesail client returned 401."
curl --fail --silent --show-error --max-time 20 --netrc-file "$work/netrc" \
    "$origin/proxy/dashboard/" > "$work/dashboard"
echo "PASS: authenticated dashboard request returned 200."
curl --fail --silent --show-error --location --max-redirs 5 --max-time 30 --netrc-file "$work/netrc" \
    "$origin/proxy/default/wiki/Main_Page" > "$work/wiki"
grep -q 'Main Page' "$work/wiki"
echo "PASS: wiki Main Page and redirects work through the real tunnel."
curl --fail --silent --show-error --max-time 20 --netrc-file "$work/netrc" \
    "$origin/proxy/default/w/api.php?action=query&meta=siteinfo&siprop=general&format=json" > "$work/siteinfo"
python3 - "$work/siteinfo" "$origin/proxy/default" <<'PY'
import json, sys
actual = json.load(open(sys.argv[1]))['query']['general']['server']
assert actual == sys.argv[2], (actual, sys.argv[2])
print('PASS: MediaWiki canonical URL uses the client HTTP origin and proxy prefix.')
PY
echo "PASS: real Holesail server and separate client container verified on this VM."
