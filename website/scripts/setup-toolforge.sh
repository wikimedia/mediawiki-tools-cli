#!/bin/sh
set -eu

# Run in the Toolforge tool account, after uploading website/dist to public_html.
action=${1:-start}
case "$action" in
  start|restart) ;;
  *) echo 'Usage: sh setup-toolforge.sh [start|restart]' >&2; exit 1 ;;
esac

mkdir -p "$HOME/bin"
download=$(mktemp "$HOME/bin/caddy.XXXXXX")
trap 'rm -f "$download"' EXIT HUP INT TERM

curl -fL \
  'https://caddyserver.com/api/download?os=linux&arch=amd64' \
  -o "$download"

chmod +x "$download"
mv "$download" "$HOME/bin/caddy"

cat > "$HOME/Caddyfile" <<'EOF'
{
  admin off
}

:{$PORT:8000} {
  root * "{$HOME}/public_html"

  # Preserve separate HTML and Markdown variants in intermediary caches.
  header Vary Accept
  @markdown_file {
    path *.md
    file {path}
  }
  header @markdown_file Content-Type "text/markdown; charset=utf-8"

  map {path} {markdown_path} {
    / /index.md
    ~^(.*?)/?$ ${1}.md
  }
  @markdown {
    # An explicit Markdown media type with nonzero quality (or no quality).
    header_regexp Accept `(?i)(?:^|,)\s*text/markdown\s*(?:;\s*q=(?:1(?:\.0{1,3})?|0?\.\d*[1-9]\d*))?\s*(?:,|$)`
    file {markdown_path}
  }
  handle @markdown {
    rewrite * {markdown_path}
    header Content-Type "text/markdown; charset=utf-8"
    file_server
  }
  file_server

  handle_errors 404 {
    rewrite * /404.html
    file_server
  }
}
EOF

cat > "$HOME/serve-docs.sh" <<'EOF'
#!/bin/sh
exec "$HOME/bin/caddy" run --config "$HOME/Caddyfile" --adapter caddyfile
EOF

chmod +x "$HOME/serve-docs.sh"

webservice --backend=kubernetes trixie "$action" "$HOME/serve-docs.sh"
