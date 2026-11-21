# Live recording. Run with bash ./docs/guides/dumps.record.sh.
Output "_recordings/dumps.gif"
Output "_recordings/dumps.mp4"
Output "_recordings/dumps.webm"
Output "_recordings/dumps.txt"

Require wm
Require grep
Set Shell "bash"
Set Theme "Dracula"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 18
Set Width 1600
Set Height 900
Set Padding 20
Set Framerate 15
Set TypingSpeed 22ms

Hide
Type 'clear'
Enter
Wait+Line@10s /^> ?$/
Show

Type '# Find a smaller Wikidata dump'
Enter
Type 'wm dumps datasets'
Enter
Sleep 250ms
Wait+Line@15s /^> ?$/
Sleep 3s
Type "wm dumps list sql/wikidatawiki/20261101 | grep -E 'Path|iwlinks.sql.gz '"
Enter
Sleep 250ms
Wait+Line@45s /^> ?$/
Sleep 3s
Type 'dump=wikidatawiki/20261101/wikidatawiki-20261101-iwlinks.sql.gz'
Enter
Sleep 1s
Ctrl+L

Type '# Compare the same file on Wikimedia and Umea (up to 32 MiB / 10s each)'
Enter
Type 'wm dumps mirrors test "$dump" wikimedia umu --yes'
Enter
Sleep 250ms
Wait+Line@45s /^> ?$/
Sleep 5s
Ctrl+L

Type '# Download the 21 MiB SQL dump, then interrupt it'
Enter
Type 'wm dumps download "$dump" --file wikidata-iwlinks.sql.gz'
Enter
Wait+Screen@45s /\/ 22447993 bytes/
Sleep 700ms
Ctrl+C
Sleep 250ms
Wait+Line@15s /^> ?$/
Sleep 3s
Hide
Type 'test -s wikidata-iwlinks.sql.gz.part && test ! -e wikidata-iwlinks.sql.gz && touch interrupted || exit 1'
Enter
Wait+Line@10s /^> ?$/
Ctrl+L
Show

Type '# Resume from the saved partial file instead of starting again'
Enter
Type 'wm dumps download "$dump" --file wikidata-iwlinks.sql.gz --resume'
Enter
Sleep 250ms
Wait+Line@90s /^> ?$/
Sleep 4s
Screenshot "_recordings/dumps.png"
