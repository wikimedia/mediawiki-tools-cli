# Run with CONTEXT=recording ./docs/guides/dev/api.record.sh.
Output "_recordings/dev-api.gif"
Output "_recordings/dev-api.mp4"
Output "_recordings/dev-api.webm"
Output "_recordings/dev-api.txt"

Require mw

Set Shell "bash"
Set Theme "Dracula"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 20
Set Width 1440
Set Height 900
Set Padding 20
Set Framerate 15
Set TypingSpeed 25ms

Hide
Type 'mw() { if [[ "$1" == api ]]; then shift; docker run --rm --network "$MWCLI_RECORD_NETWORK" -v "$MWCLI_RECORD_BIN:/mw:ro" --entrypoint /mw "$MWCLI_RECORD_IMAGE" --no-interaction api "$@"; else "$MWCLI_RECORD_BIN" --no-interaction "$@"; fi; }'
Enter
Type 'export LOCAL_WIKI=http://mediawiki-web:8080'
Enter
Type 'clear'
Enter
Show

Type '# Read a page with the REST API'
Enter
Type 'mw api "$LOCAL_WIKI/w/rest.php/v1/page/Main_Page" --output jq --format "{title, latest: .latest.timestamp}"'
Enter
Wait+Line@45s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Ask the Action API about this wiki'
Enter
Type 'mw api "$LOCAL_WIKI/w/api.php?action=query&meta=siteinfo&format=json&formatversion=2" --output jq --format ".query.general | {sitename, mainpage, dbtype, generator}"'
Enter
Wait+Line@45s /^> ?$/
Sleep 2s
