#!/usr/bin/env bash
# Run against the wmf-wikifarm recipe in a disposable development context.
# CONTEXT=wmf-farm bash tests/test-docker-recipe-wmf-wikifarm.sh
set -euo pipefail

source "$(dirname -- "${BASH_SOURCE[0]}")/functions.sh"
MWCLI_BIN=$(test_cli_binary "${MWCLI_BIN:-${MW_BIN:-}}")
: "${CONTEXT:?Select a disposable context running the wmf-wikifarm recipe}"

compose() { "$MWCLI_BIN" --no-interaction --context "$CONTEXT" dev compose "$@"; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
compose cp -- farm-https:/data/caddy/pki/authorities/local/root.crt "$work/ca.crt"
request() { curl --cacert "$work/ca.crt" --no-progress-meter --fail --max-time 30 "$@"; }
api() {
    local host=$1
    shift
    request --get "https://$host:8443/w/api.php" --data 'format=json&formatversion=2' "$@"
}

api en.wikipedia.localhost --data 'action=query&meta=siteinfo&siprop=general|extensions' | jq -e '
    .query.general.wikiid == "enwiki" and
    (.query.extensions | map(.name) | contains(["CentralAuth", "CirrusSearch", "ContentTranslation", "Scribunto"]))'
api de.wikipedia.localhost --data 'action=query&meta=siteinfo' | jq -e '.query.general.lang == "de"'
api www.wikidata.localhost --data 'action=wbgetentities&ids=Q1' | jq -e '.entities.Q1.id == "Q1"'

login=$(request --location --cookie-jar "$work/cookies" --output /dev/null --write-out '%{url_effective}' \
    'https://en.wikipedia.localhost:8443/wiki/Special:UserLogin')
[[ "$login" == https://auth.wikimedia.localhost:8443/enwiki/* ]]
request 'https://office.wikimedia.localhost:8443/wiki/Private_farm_page' > "$work/office.html"
if grep -Fq 'FARM-PRIVATE-MARKER' "$work/office.html"; then
    echo 'Private wiki content is visible to an anonymous request' >&2
    exit 1
fi
request 'https://vote.wikimedia.localhost:8443/wiki/Fishbowl_farm_page' > "$work/vote.html"
grep -Fq 'This page is public' "$work/vote.html"

for ((attempt = 1; attempt <= 30; attempt++)); do
    if api en.wikipedia.localhost --data 'action=query&list=search' --data-urlencode 'srsearch="job runner updates search"' |
        jq -e 'any(.query.search[]; .title == "Wiki farm test page")' >/dev/null; then
        break
    fi
    if ((attempt == 30)); then
        echo 'The farm fixture did not reach the search index' >&2
        exit 1
    fi
    sleep 1
done

api en.wikipedia.localhost --data 'action=parse&prop=text' --data-urlencode 'page=Wiki farm test chart' |
    jq -e '.parse.text | contains("wiki-chart")'
api en.wikipedia.localhost --data 'action=parse&prop=text' --data-urlencode 'page=Wiki farm rendering test' |
    jq -e '.parse.text | contains("Hello from Lua") and contains("Hello from Shellbox") and contains("mw-highlight")'
api en.wikipedia.localhost --data 'action=parse&prop=text&title=Shellbox_smoke&contentmodel=wikitext' \
    --data-urlencode "text=<syntaxhighlight lang=\"python\">print(\"Shellbox smoke $$\")</syntaxhighlight>" |
    jq -e '.parse.text | contains("Shellbox smoke") and contains("mw-highlight")'

awk '{ printf "%s ", $0 } END { print "" }' <<'PHP' |
$services = MediaWiki\MediaWikiServices::getInstance();
$clients = $services->getShellboxClientFactory();
$commands = $services->getShellCommandFactory();
foreach ( [ 'pdfhandler', 'constraint-regex-checker', 'score', 'syntaxhighlight', 'easytimeline' ] as $name ) {
    if ( !$clients->isEnabled( $name ) ) {
        exit( 1 );
    }
    $result = $commands->createBoxed( $name )->routeName( 'wmcli-smoke' )->params( '/bin/echo', 'ok' )->execute();
    if ( $result->getExitCode() !== 0 || trim( $result->getStdout() ) !== 'ok' ) {
        exit( 1 );
    }
    echo "Shellbox $name passed\n";
}
PHP
    compose exec -- -T mediawiki php maintenance/run.php eval --wiki enwiki > "$work/shellbox.log"
grep -Fx 'Shellbox easytimeline passed' "$work/shellbox.log"

request 'https://cxserver.wikimedia.localhost:8443/v1/list/mt/en/de' | jq -e '.mt | index("TestClient") != null'
request 'https://cxserver.wikimedia.localhost:8443/v1/page/en/Wiki_farm_test_page' |
    jq -e '.sourceLanguage == "en" and (.segmentedContent | contains("job runner updates search"))'

image="Wiki-farm-smoke-$(date +%s)-$$.png"
compose exec -- -T mediawiki sh -ec '
    fixture=$(mktemp -d)
    trap '\''rm -rf "$fixture"'\'' EXIT
    php -r '\''
        $image = imagecreatetruecolor(128, 128);
        imagefilledellipse($image, 64, 64, 64, 64, imagecolorallocate($image, 0, 170, 150));
        imagepng($image, $argv[1]);
    '\'' -- "$fixture/$1"
    php maintenance/run.php importImages --wiki commonswiki --user FarmUser "$fixture"
' sh "$image"

file_info=$(api en.wikipedia.localhost --data 'action=query&prop=imageinfo&iiprop=url&iiurlwidth=64' --data-urlencode "titles=File:$image")
jq -e '.query.pages[0].imagerepository == "shared"' <<< "$file_info"
original=$(jq -er '.query.pages[0].imageinfo[0].url | select(startswith("https://upload.wikimedia.localhost:8443/wikipedia/commons/"))' <<< "$file_info")
thumbnail=$(jq -er '.query.pages[0].imageinfo[0].thumburl | select(startswith("https://upload.wikimedia.localhost:8443/wikipedia/commons/thumb/"))' <<< "$file_info")
[[ $(request --output /dev/null --write-out '%{content_type}' "$original") == image/png ]]
[[ $(request --output /dev/null --write-out '%{content_type}' "$thumbnail") == image/png ]]

echo 'Wikimedia wiki farm smoke checks passed'
