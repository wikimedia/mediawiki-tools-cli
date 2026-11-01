#!/bin/sh
set -eu

# Run in the Toolforge tool account, after uploading website/dist to public_html.
mkdir -p "$HOME/bin"
download=$(mktemp "$HOME/bin/caddy.XXXXXX")
trap 'rm -f "$download"' EXIT HUP INT TERM

curl -fL \
  'https://caddyserver.com/api/download?os=linux&arch=amd64' \
  -o "$download"

chmod +x "$download"
mv "$download" "$HOME/bin/caddy"

cat > "$HOME/serve-docs.sh" <<'EOF'
#!/bin/sh
exec "$HOME/bin/caddy" file-server \
  --root "$HOME/public_html" \
  --listen ":${PORT:-8000}"
EOF

chmod +x "$HOME/serve-docs.sh"

webservice --backend=kubernetes trixie start "$HOME/serve-docs.sh"
