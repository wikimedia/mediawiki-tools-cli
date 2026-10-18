# MWCLI web interface assets

The dashboard and gateway login/logout pages use the official CSS-only
components from Wikimedia Codex 2.7.0. Both containers mount this directory at
`/ui`, and serve the styles themselves; no CDN, Node.js, or Vue runtime is needed.

`codex.style.css`, `tokens.css`, and `tokens-dark.css` are unmodified npm release
assets from `@wikimedia/codex` and `@wikimedia/codex-design-tokens`. Their MIT
licenses are included alongside them. `app.css` supplies only application layout
and typography using Codex design tokens. Component styling comes from Codex.

To refresh the vendored files from the pinned releases, run:

```sh
npm ci --prefix tools/ui
python3 tools/ui/update-codex.py
make generate
```

Updating assets requires Python 3 and Node.js 18 or later. The update script
also compiles Codex's official Link Less mixin to `codex-links.css`, since link
styles are not included in the component stylesheet. The Less compiler is pinned
in `tools/ui/package-lock.json`; no npm dependencies are shipped to the browser.

When upgrading Codex, update the version and the npm tarball SHA-512 integrity
values in the script, and review the component markup against the release docs:
https://doc.wikimedia.org/codex/main/using-codex/developing.html

For standalone development, set `MWCLI_UI_ASSETS` to this directory before
running either Go server. Otherwise the servers read `/ui` as configured by
Compose. The dark token theme follows the browser's preferred color scheme.
