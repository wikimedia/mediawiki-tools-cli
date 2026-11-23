#!/usr/bin/env bash
# Exercise opt-in Swift storage and Thumbor against an already prepared .mediawiki.
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/functions.sh"
MW_BIN=$(test_cli_binary "${MW_BIN:-}")
export MWCLI_CONTEXT_TEST=1
work=$(mktemp -d)
context="storage-$$-${RANDOM}"
dev() { "$MW_BIN" --no-interaction dev --context "$context" "$@"; }
compose() { dev compose "$@"; }
cleanup() {
    local result=$?
    trap - EXIT
    if ((result)); then compose logs -- --tail=100 >&2 || true; fi
    dev destroy || result=1
    dev env clear || result=1
    rm -rf "$work"
    exit "$result"
}
trap cleanup EXIT
cp -a --reflink=auto "$SCRIPT_DIR/../.mediawiki" "$work/mediawiki"
# Only the disposable copy is reset; never mutate the caller's checkout.
rm -rf "$work/mediawiki/LocalSettings.d" "$work/mediawiki/cache/docker"
dev env set MEDIAWIKI_VOLUMES_CODE "$work/mediawiki"
dev mediawiki create
for wiki in storage privatestorage; do dev mediawiki install --dbtype=sqlite --dbname="$wiki"; done
mkdir -p "$work/mediawiki/LocalSettings.d/privatestorage"
printf '%s\n' '<?php $wgGroupPermissions["*"]["read"] = false;' > "$work/mediawiki/LocalSettings.d/privatestorage/private.php"
port=$(dev env get PORT)
base="http://127.0.0.1:$port"
request() {
    local wiki=$1
    shift
    curl --silent --show-error --connect-timeout 3 --max-time 30 \
        -H "Host: $wiki.mediawiki.local.wmftest.net:$port" "$@"
}
api() {
    local wiki=$1
    shift
    request "$wiki" --fail --get "$base/w/api.php" --data 'format=json&formatversion=2' "$@"
}
evaluate() { compose exec -- -T mediawiki php maintenance/run.php eval --wiki "$1"; }
wait_api() {
    for ((attempt=0; attempt<30; attempt++)); do
        if api storage --data 'action=query&meta=siteinfo' | jq -e '.query.general.wikiid == "storage"' >/dev/null; then return; fi
        sleep 2
    done
    echo 'MediaWiki API did not become ready' >&2
    return 1
}
image_url() {
    local wiki=$1 width=$2
    shift 2
    api "$wiki" "$@" --data 'action=query&prop=imageinfo&iiprop=url' --data "iiurlwidth=$width" \
        --data-urlencode "titles=File:$image"
}
fetch_url() {
    local wiki=$1 url=$2
    shift 2
    local path
    path=$(python3 -c 'import sys, urllib.parse; u=urllib.parse.urlsplit(sys.argv[1]); print(urllib.parse.urlunsplit(("", "", u.path, u.query, "")))' "$url")
    request "$wiki" "$base$path" "$@"
}
assert_png() {
    python3 - "$1" "$2" "$3" <<'PY'
import struct, sys
with open(sys.argv[1], 'rb') as image:
    header = image.read(24)
assert header[:8] == b'\x89PNG\r\n\x1a\n', 'Expected a PNG response'
assert struct.unpack('>II', header[16:24]) == (int(sys.argv[2]), int(sys.argv[3])), 'Wrong thumbnail dimensions'
PY
}
upload_image() {
    compose exec -- -T mediawiki sh -ec '
        dir=$(mktemp -d)
        trap '\''rm -rf "$dir"'\'' EXIT
        php -r '\''$i = imagecreatetruecolor(128, 96); imagefilledrectangle($i, 0, 0, 127, 95, imagecolorallocate($i, 20, 160, 100)); imagepng($i, $argv[1]);'\'' "$dir/$1"
        php maintenance/run.php importImages --wiki "$2" --user admin "$dir"
    ' sh "$image" "$1"
}

# A running storage service must not switch existing wikis away from filesystem.
dev swift create
for ((attempt=0; attempt<30; attempt++)); do
    if compose exec -- -T swift /usr/local/bin/healthcheck.sh >/dev/null 2>&1; then break; fi
    if ((attempt == 29)); then echo 'Swift did not become ready' >&2; exit 1; fi
    sleep 3
done
printf '%s\n' '$services = MediaWiki\MediaWikiServices::getInstance(); if ( $services->getRepoGroup()->getLocalRepo()->getBackend() instanceof Wikimedia\FileBackend\SwiftFileBackend ) { exit(1); }' |
    evaluate storage

dev env set MEDIAWIKI_SWIFT_WIKIS storage,privatestorage
dev env set MEDIAWIKI_SWIFT_PRIVATE_WIKIS privatestorage
dev env set MEDIAWIKI_THUMBOR_WIKIS ''
dev mediawiki create --force-recreate
for wiki in storage privatestorage; do
    for repeat in 1 2; do
        compose exec -- -T mediawiki php maintenance/run.php /wm-swift/prepareSwift.php --wiki "$wiki"
    done
done
image="Storage-smoke-$$.png"
for wiki in storage privatestorage; do
    upload_image "$wiki"
done

# Swift alone supports both original objects and locally generated thumbnails.
wait_api
info=$(image_url storage 64)
original=$(jq -er '.query.pages[0].imageinfo[0].url' <<< "$info")
thumb=$(jq -er '.query.pages[0].imageinfo[0].thumburl' <<< "$info")
fetch_url storage "$original" --fail --output "$work/original.png"
assert_png "$work/original.png" 128 96
fetch_url storage "$thumb" --fail --output "$work/local-thumb.png"
assert_png "$work/local-thumb.png" 64 48
dev swift stop
dev swift start
for ((attempt=0; attempt<30; attempt++)); do
    if fetch_url storage "$original" --fail --output "$work/persisted.png"; then break; fi
    if ((attempt == 29)); then echo 'Swift object did not survive restart' >&2; exit 1; fi
    sleep 3
done
cmp "$work/original.png" "$work/persisted.png"

# Enable Thumbor explicitly, requesting a new size to avoid the local thumbnail.
dev thumbor create
dev env set MEDIAWIKI_THUMBOR_WIKIS storage,privatestorage
dev mediawiki create --force-recreate
printf '%s\n' 'if ( !$wgLocalFileRepo["disableLocalTransform"] || !$wgLocalFileRepo["transformVia404"] ) { exit(1); }' |
    evaluate storage
wait_api
info=$(image_url storage 48)
thumb=$(jq -er '.query.pages[0].imageinfo[0].thumburl' <<< "$info")
for ((attempt=0; attempt<30; attempt++)); do
    if fetch_url storage "$thumb" --fail --output "$work/thumbor.png"; then break; fi
    if ((attempt == 29)); then echo 'Thumbor did not render the image' >&2; exit 1; fi
    sleep 2
done
assert_png "$work/thumbor.png" 48 36

# Log into the private wiki through MediaWiki, using a separate cookie jar.
cookies="$work/private.cookies"
token=$(api privatestorage --cookie-jar "$cookies" --data 'action=query&meta=tokens&type=login' | jq -er '.query.tokens.logintoken')
request privatestorage --fail --cookie "$cookies" --cookie-jar "$cookies" "$base/w/api.php" \
    --data 'action=login&format=json&lgname=admin&lgpassword=mwddpassword' --data-urlencode "lgtoken=$token" |
    jq -e '.login.result == "Success"'
info=$(image_url privatestorage 48 --cookie "$cookies")
for field in url thumburl; do
    url=$(jq -er --arg field "$field" '.query.pages[0].imageinfo[0][$field]' <<< "$info")
    status=$(fetch_url privatestorage "$url" --output "$work/anonymous-$field" --write-out '%{http_code}')
    [[ $status == 401 || $status == 403 || $status == 404 ]]
    fetch_url privatestorage "$url" --cookie "$cookies" --fail --output "$work/private-$field.png"
done
assert_png "$work/private-url.png" 128 96
assert_png "$work/private-thumburl.png" 48 36
# Bypassing img_auth must not expose the private object through Swift's public URL.
private_original=$(jq -er '.query.pages[0].imageinfo[0].url' <<< "$info")
private_path=${private_original#*/img_auth.php/}
status=$(request privatestorage "$base/proxy/swift/wikipedia/privatestorage/$private_path" \
    --output /dev/null --write-out '%{http_code}')
[[ $status == 401 || $status == 403 || $status == 404 ]]
status=$(request privatestorage "$base/proxy/swift/wikipedia/privatestorage/thumb/$private_path/48px-$image" \
    --output /dev/null --write-out '%{http_code}')
[[ $status == 401 || $status == 403 || $status == 404 ]]
echo 'Swift storage, restart persistence, Thumbor resizing and private access checks passed'
