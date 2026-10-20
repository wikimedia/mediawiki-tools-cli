# Run with CONTEXT=recording-setup-final ./docs/guides/dev/extension-workflow.record.sh.
Output "_recordings/dev-extension-workflow.gif"
Output "_recordings/dev-extension-workflow.mp4"
Output "_recordings/dev-extension-workflow.webm"
Output "_recordings/dev-extension-workflow.txt"

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
Type 'cd "$(mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki where)"'
Enter
Type 'clear'
Enter
Show

Type '# Get an extension into this checkout'
Enter
Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki get-code --extension Cite --gerrit-interaction-type http --no-commit-hook'
Enter
Wait+Line@90s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Enable it in LocalSettings.php'
Enter
Type 'cat >> LocalSettings.php <<EOF'
Enter
Type "wfLoadExtension( 'Cite' );"
Enter
Type 'EOF'
Enter
Sleep 1s
Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki mwscript update'
Enter
Wait+Line@90s /^> ?$/
Sleep 2s
Ctrl+L

Type '# Run a focused extension test'
Enter
Type 'mw dev --context "$MWCLI_RECORD_CONTEXT" mediawiki composer phpunit extensions/Cite/tests/phpunit/unit/CiteParserTagHooksTest.php'
Enter
Wait+Line@90s /^> ?$/
Sleep 3s
