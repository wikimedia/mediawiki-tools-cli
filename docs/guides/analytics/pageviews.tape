# Run with ./docs/guides/analytics/pageviews.record.sh.
Output "_recordings/pageviews.gif"
Output "_recordings/pageviews.mp4"
Output "_recordings/pageviews.webm"
Output "_recordings/pageviews.txt"

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

Type '# Graph the same Wikimedia pageview data'
Enter
Type 'mw analytics pageviews "Albert Einstein" --start 20261001 --end 20261003 --output graph'
Enter
Wait+Line@45s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Use JSON in a script or jq'
Enter
Type 'mw analytics pageviews "Albert Einstein" --start 20261001 --end 20261003 --output json'
Enter
Wait+Line@45s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Export rows for a spreadsheet'
Enter
Type 'mw analytics pageviews "Albert Einstein" --start 20261001 --end 20261003 --output csv'
Enter
Wait+Line@45s /^> ?$/
Sleep 2s
