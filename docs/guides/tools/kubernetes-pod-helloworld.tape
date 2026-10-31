# Run with TOOL=your-tool ./docs/guides/tools/kubernetes-pod-helloworld.record.sh.
Output "_recordings/kubernetes-pod-helloworld.gif"
Output "_recordings/kubernetes-pod-helloworld.mp4"
Output "_recordings/kubernetes-pod-helloworld.webm"
Output "_recordings/kubernetes-pod-helloworld.txt"

Require mw
Require curl

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
Type 'set -e; cd _recordings; clear'
Enter
Wait+Line@120s /^> ?$/
Show
Type '# Inspect the tool before changing anything'
Enter
Type 'echo "Tool: $TOOL"'
Enter
Sleep 1s
Type 'mw toolforge kubernetes get pods --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes get deployments --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes get services --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s

Ctrl+L
Type '# The manifest comes from the guide'
Enter
Type 'cat cli-guide-hello.yaml'
Enter
Sleep 5s

Ctrl+L
Type 'mw toolforge kubernetes create --dry-run=server -f cli-guide-hello.yaml --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes create -f cli-guide-hello.yaml --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes wait --for=condition=Ready pod/cli-guide-hello --timeout=120s --tool "$TOOL"'
Enter
Hide
Wait+Line@150s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes get pod cli-guide-hello --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes logs cli-guide-hello --tail=2 --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes exec cli-guide-hello --tool "$TOOL" -- python3 --version'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s

Ctrl+L
Type '# Forward the app to localhost'
Enter
Type 'mw toolforge kubernetes port-forward pod/cli-guide-hello 18080:8000 --tool "$TOOL" > port-forward.log 2>&1 &'
Enter
Hide
Type 'forward_pid=$!; for i in {1..30}; do grep -q "Forwarding from" port-forward.log && break; kill -0 "$forward_pid" || exit 1; sleep 1; done; grep -q "Forwarding from" port-forward.log'
Enter
Wait+Line@45s /^> ?$/
Type 'clear'
Enter
Wait+Line@10s /^> ?$/
Show
Type 'head -n 2 port-forward.log'
Enter
Sleep 2s
Type 'curl --fail http://127.0.0.1:18080/'
Enter
Wait+Line@30s /^> ?$/
Sleep 4s
Screenshot "_recordings/kubernetes-pod-helloworld.png"
Sleep 1s

Hide
Type 'kill "$forward_pid"; wait "$forward_pid" 2>/dev/null || true; clear'
Enter
Wait+Line@15s /^> ?$/
Show
Type '# Remove only the demo pod'
Enter
Type 'mw toolforge kubernetes delete pod cli-guide-hello --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 2s
Type 'mw toolforge kubernetes get pods --tool "$TOOL"'
Enter
Hide
Wait+Line@120s /^> ?$/
Show
Sleep 4s
