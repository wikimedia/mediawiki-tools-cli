#!/usr/bin/env bash
# Run against an installed wiki in a disposable context:
# CONTEXT=logs-test MWCLI_BIN=./bin/mw bash tests/test-docker-logs.sh
set -euo pipefail

MWCLI_BIN=${MWCLI_BIN:-./bin/mw}
: "${CONTEXT:?Select a disposable development context with an installed default wiki}"

mw() { "$MWCLI_BIN" --no-interaction --context "$CONTEXT" "$@"; }
compose() { mw dev compose "$@"; }
search() {
    compose exec -- -T logs-opensearch curl --fail --silent --show-error --max-time 20 \
        -H 'Content-Type: application/json' "http://localhost:9200/$1" "${@:2}"
}
dashboard() {
    compose exec -- -T opensearch-dashboards curl --fail --silent --show-error --max-time 20 \
        -H 'osd-xsrf: mwcli-test' -H 'Content-Type: application/json' \
        "http://localhost:5601/proxy/logs/$1" "${@:2}"
}
show_failure() {
    status=$?
    if [ "$status" -ne 0 ]; then
        compose logs -- --tail=80 logs-setup logs-collector logs-opensearch opensearch-dashboards || true
    fi
    exit "$status"
}
trap show_failure EXIT

mw dev logs create

dashboard api/saved_objects/index-pattern/mwcli-logs | jq -e \
    '.attributes.title == "mwcli-logs-*" and .attributes.timeFieldName == "@timestamp"'
dashboard api/saved_objects/search/mwcli-mediawiki-logs | jq -e \
    '.attributes.columns | index("message") != null'
echo 'PASS: log index and saved MediaWiki view are provisioned'

port=$(mw dev env get PORT)
probe="mwcli-logs-test-$(date +%s)-$$"
request_wiki() {
    curl --fail --silent --show-error --max-time 60 -H "Host: default.mediawiki.local.wmftest.net:$port" \
        "http://127.0.0.1:$port/w/api.php?action=query&meta=siteinfo&format=json&mwcli_probe=$1" \
        | jq -e '.query.general.sitename'
}
request_wiki "$probe"
query=$(jq -nc --arg probe "$probe" \
    '{size:0, query:{match_phrase:{message:$probe}}, aggs:{sources:{terms:{field:"source.keyword"}}}}')
found=false
for ((attempt=0; attempt<60; attempt++)); do
    result=$(search 'mwcli-logs-*/_search' -d "$query")
    if jq -e '[.aggregations.sources.buckets[].key] | index("container") != null and index("mediawiki_file") != null' \
        <<< "$result" >/dev/null; then
        found=true
        break
    fi
    sleep 2
done
if [ "$found" != true ]; then
    echo "FAIL: the real wiki request did not reach both log sources: $result" >&2
    exit 1
fi
echo 'PASS: a real wiki request is searchable in both MediaWiki files and container output'

search _plugins/_ppl -d '{"query":"source = `mwcli-logs-*` | where service = '\''mediawiki'\'' | head 1"}' \
    | jq -e '.datarows | length > 0'
echo 'PASS: the Observability Logs source query returns MediaWiki logs'

# Starting again must retain edits to the saved view and user preferences.
dashboard api/saved_objects/search/mwcli-mediawiki-logs -X PUT \
    -d '{"attributes":{"title":"My MediaWiki logs"}}' >/dev/null
dashboard api/opensearch-dashboards/settings -X POST \
    -d '{"changes":{"defaultRoute":"/app/discover"}}' >/dev/null
compose run -- --rm --no-deps logs-setup
dashboard api/saved_objects/search/mwcli-mediawiki-logs | jq -e '.attributes.title == "My MediaWiki logs"'
dashboard api/opensearch-dashboards/settings | jq -e '.settings.defaultRoute.userValue == "/app/discover"'
echo 'PASS: repeated setup preserves saved-view edits and user settings'

mw dev logs stop
request_wiki "${probe}-stopped"
compose logs -- --tail=20 mediawiki-web | grep -F "${probe}-stopped"
mw dev logs create
echo 'PASS: the wiki and normal Docker logs keep working when log browsing stops'
