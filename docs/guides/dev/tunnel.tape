# Run with CONTEXT=recording-setup-final ./docs/guides/dev/tunnel.record.sh.
Output "_recordings/dev-tunnel.gif"
Output "_recordings/dev-tunnel.mp4"
Output "_recordings/dev-tunnel.webm"
Output "_recordings/dev-tunnel.txt"

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

Type '# Open a temporary HTTPS tunnel to the local gateway'
Enter
Type 'mw dev tunnel start'
Enter
Wait+Line@180s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Check the tunnel and copy its URL'
Enter
Type 'mw dev tunnel status'
Enter
Wait+Line@45s /^> ?$/
Type 'mw dev tunnel url'
Enter
Sleep 2s
Ctrl+L

Type '# Copy the HTTPS URL and sign in through the gateway'
Enter
Type '# Stop the public tunnel when finished'
Enter
Type 'mw dev tunnel stop'
Enter
Wait+Line@90s /^> ?$/
Sleep 3s
