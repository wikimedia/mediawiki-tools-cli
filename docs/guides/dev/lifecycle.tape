# Run with CONTEXT=recording-setup-final ./docs/guides/dev/lifecycle.record.sh.
Output "_recordings/dev-lifecycle.gif"
Output "_recordings/dev-lifecycle.mp4"
Output "_recordings/dev-lifecycle.webm"
Output "_recordings/dev-lifecycle.txt"

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
Type 'export CONTEXT="$MWCLI_RECORD_CONTEXT"'
Enter
Type 'mw() { "$MWCLI_RECORD_BIN" --no-interaction "$@"; }'
Enter
Type 'clear'
Enter
Show

Type '# Stop the running environment'
Enter
Type 'mw dev stop'
Enter
Wait+Line@90s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Start the same services again'
Enter
Type 'mw dev start'
Enter
Wait+Line@120s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Restart and inspect service health'
Enter
Type 'mw dev restart'
Enter
Wait+Line@120s /^> ?$/
Type 'mw dev compose ps'
Enter
Sleep 3s
Type 'mw dev compose logs -- --tail=5 mediawiki-web'
Enter
Wait+Line@45s /^> ?$/
Sleep 3s
