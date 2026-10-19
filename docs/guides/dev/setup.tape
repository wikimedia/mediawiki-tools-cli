# Run with MEDIAWIKI_CODE=/path/to/mediawiki CONTEXT=recording-setup \
#   ./docs/guides/dev/setup.record.sh
Output "_recordings/dev-setup.gif"
Output "_recordings/dev-setup.mp4"
Output "_recordings/dev-setup.webm"
Output "_recordings/dev-setup.txt"

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
Type 'mw() { "$MWCLI_RECORD_BIN" --no-interaction "$@"; }'
Enter
Type 'clear'
Enter
Show

Type '# Start a local MediaWiki environment'
Enter
Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki create >/dev/null 2>&1 && echo "MediaWiki services started"'
Enter
Hide
Wait+Line@180s /^> ?$/
Show
Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" mysql create >/dev/null 2>&1 && echo "MySQL is ready"'
Enter
Hide
Wait+Line@180s /^> ?$/
Show
Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki install --dbtype=mysql >/dev/null 2>&1 && echo "Local wiki installed"'
Enter
Hide
Wait+Line@240s /^> ?$/
Show

Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" compose ps'
Enter
Sleep 3s
