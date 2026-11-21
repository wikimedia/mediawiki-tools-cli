# Run with ./docs/guides/tools/wiki-replicas.record.sh.
# Scripted replay of supplied output, not a live database session.
Output "_recordings/toolforge-sql.gif"
Output "_recordings/toolforge-sql.mp4"
Output "_recordings/toolforge-sql.webm"
Output "_recordings/toolforge-sql.txt"

Require wm
Set Shell "bash"
Set Theme "Dracula"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 18
Set Width 1600
Set Height 700
Set Padding 20
Set Framerate 15
Set TypingSpeed 25ms

Hide
Type 'set -e; clear'
Enter
Sleep 250ms
Wait+Line@10s /^> ?$/
Show

Type '# Query a Wiki Replica using existing Toolforge credentials'
Enter
Type "wm toolforge replicas sql itwiki -- -e 'SELECT COUNT(*) FROM user'"
Enter
Sleep 250ms
Wait+Line@10s /^> ?$/
Sleep 4s

Ctrl+L
Type '# Discover a tool database and inspect its tables'
Enter
Type 'wm toolforge toolsdb list --tool wikicrowd'
Enter
Sleep 250ms
Wait+Line@10s /^> ?$/
Sleep 2s
Type "wm toolforge toolsdb sql s54943__laravel --tool wikicrowd -- -e 'SHOW tables'"
Enter
Sleep 250ms
Wait+Line@10s /^> ?$/
Sleep 4s

Ctrl+L
Type '# Query ToolsDB as a tool account'
Enter
Type "wm toolforge toolsdb sql s54943__laravel --tool wikicrowd -- -e 'SELECT COUNT(*) from answers'"
Enter
Sleep 250ms
Wait+Line@10s /^> ?$/
Sleep 5s
Screenshot "_recordings/toolforge-sql.png"
