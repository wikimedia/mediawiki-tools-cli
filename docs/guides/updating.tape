# Run with ./docs/guides/updating.record.sh.
Output "_recordings/update.gif"
Output "_recordings/update.mp4"
Output "_recordings/update.webm"
Output "_recordings/update.txt"

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

Type '# Check the installed version'
Enter
Type 'mw version'
Enter
Sleep 2s
Ctrl+L

Type '# Preview v0.33.0 and its changelog without installing it'
Enter
Type 'mw update --dry-run --version v0.33.0'
Enter
Wait+Line@90s /^> ?$/
Sleep 3s
