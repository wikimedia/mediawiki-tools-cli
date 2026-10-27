# Run with CONTEXT=recording ./docs/guides/dev/mysql-dumps.record.sh.
Output "_recordings/dev-mysql-dump.mp4"
Output "_recordings/dev-mysql-dump.webm"
Output "_recordings/dev-mysql-dump.txt"

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

Type '# Export the local wiki database'
Enter
Type 'mw dev compose exec -- -T mysql sh -c "mariadb-dump -uroot -ptoor --single-transaction default > /tmp/mw-default.sql"'
Enter
Wait+Line@60s /^> ?$/
Type 'mw dev compose cp mysql:/tmp/mw-default.sql _recordings/mw-default.sql'
Enter
Wait+Line@30s /^> ?$/
Type 'ls -lh _recordings/mw-default.sql'
Enter
Sleep 2s

Type '# Restore a copy into a second local database'
Enter
Type 'mw dev compose exec -- -T mysql mariadb -uroot -ptoor -e "CREATE DATABASE recording_restore"'
Enter
Wait+Line@30s /^> ?$/
Type 'mw dev compose exec -- -T mysql mariadb -uroot -ptoor recording_restore < _recordings/mw-default.sql'
Enter
Wait+Line@60s /^> ?$/
Type 'mw dev compose exec -- -T mysql mariadb -uroot -ptoor -N -D recording_restore -e "SELECT COUNT(*) AS restored_tables FROM information_schema.tables WHERE table_schema=DATABASE()"'
Enter
Wait+Line@30s /^> ?$/
Sleep 2s
