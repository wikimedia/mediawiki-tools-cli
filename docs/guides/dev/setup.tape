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
Set TypingSpeed 30ms

Hide
Type 'export CONTEXT="$MWCLI_RECORD_CONTEXT"'
Enter
Type 'mw() { "$MWCLI_RECORD_BIN" --no-interaction "$@"; }'
Enter
Type 'clear'
Enter
Show

Type '# Start a local MediaWiki environment'
Enter
Type 'mw dev mediawiki create'
Enter
Sleep 250ms
Wait+Line@180s /^> ?$/
Sleep 3s
Ctrl+L
Type 'mw dev mysql create'
Enter
Sleep 250ms
Wait+Line@180s /^> ?$/
Sleep 3s
Ctrl+L
Type 'mw dev mediawiki install --dbtype=mysql'
Enter
Sleep 250ms
Wait+Line@240s /^> ?$/
Sleep 3s
Ctrl+L

Type 'mw dev compose ps'
Enter
Sleep 250ms
Wait+Line@60s /^> ?$/
Sleep 3s
