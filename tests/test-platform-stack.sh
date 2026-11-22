#!/usr/bin/env bash
# Run the same gateway and wiki checks against each container runtime.
set -euo pipefail

cli=$1
snapshot=$2
work=$(mktemp -d "$HOME/mwcli-smoke.XXXXXX")
response="$work/response"
dev() {
    "$cli" dev --context platform-smoke "$@"
}
finish() {
    local status=$?
    if ((status != 0)); then
        dev compose -- ps || true
        dev compose -- logs --tail 35 traefik docker-socket-proxy dashboard mediawiki mediawiki-web gateway-auth || true
    fi
    dev compose -- --profile gateway down --volumes --remove-orphans || true
    rm -rf -- "$work"
}
trap finish EXIT
export MWCLI_CONTEXT_TEST=1
tar -xzf "$snapshot" -C "$work"
wiki="$work/.mediawiki"
test -f "$wiki/maintenance/install.php"
rm -f "$wiki/LocalSettings.php"
printf '%s\n' '<?php echo "PHP bind mount works";' > "$wiki/probe.php"
dev env set PORT 8080
dev env set NETWORK_SUBNET_PREFIX 10.253.44
dev env set MEDIAWIKI_VOLUMES_CODE "$wiki"
dev env set MEDIAWIKI_DEFAULT_DBNAME smokewiki
if [[ -n "${MWCLI_SMOKE_DOCKER_SOCKET:-}" ]]; then
    dev env set MWCLI_DOCKER_SOCKET "$MWCLI_SMOKE_DOCKER_SOCKET"
fi
dev traefik create --no-interaction
dev compose -- up -d --no-build mediawiki gateway-auth
port=$(dev env get PORT)

probe() {
    local url=$1 expected=$2
    local deadline=$((SECONDS + ${3:-120}))
    while ((SECONDS < deadline)); do
        if curl --noproxy '*' --max-time 3 -fsS "$url" > "$response" &&
            grep -Fq -- "$expected" "$response"; then
            printf 'PASS %s\n' "$url"
            return 0
        fi
        sleep 2
    done
    printf 'Timed out waiting for %s to return %s\n' "$url" "$expected" >&2
    return 1
}

probe "http://127.0.0.1:$port/proxy/dashboard/health" '"status":"ok"' "${MWCLI_SMOKE_STARTUP_TIMEOUT:-120}"
probe "http://127.0.0.1:$port/proxy/smokewiki/w/probe.php" 'PHP bind mount works'
probe "http://127.0.0.1:$port/_wm/login" 'Sign in to the gateway' "${MWCLI_SMOKE_STARTUP_TIMEOUT:-120}"
probe "http://127.0.0.1:$port/_wm/assets/tokens.css" 'Codex Design Tokens'
probe "http://127.0.0.1:$port/proxy/dashboard/assets/tokens.css" 'Codex Design Tokens'

dev mediawiki install --dbname smokewiki --dbtype sqlite --no-interaction
probe "http://127.0.0.1:$port/proxy/smokewiki/w/api.php?action=query&meta=siteinfo&format=json" '"wikiid":"smokewiki"'
probe "http://127.0.0.1:$port/proxy/smokewiki/wiki/Main_Page" 'MediaWiki has been installed'
echo 'Gateway and MediaWiki smoke checks passed'
