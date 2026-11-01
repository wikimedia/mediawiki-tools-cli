# Documentation

The command manual and workflow guides are hosted at
https://a-wikimedia-cli.toolforge.org/. MediaWiki.org contains manually maintained
project information; releases do not publish generated pages there.

## Build and preview

Install Go and the tools pinned in `mise.toml`, then run:

```sh
make manual-install
make manual-dev
make manual-check
make manual
```

The site lives in `website/`. Commands are generated from Cobra help under
`/manual/`; Markdown guides from this directory appear under `/guides/` and are
also bundled for offline reading with `mw help guides`. `--web` opens the same
guide on the documentation site.

`make manual` writes the complete static site, search index and available media
to `website/dist/`. Building does not upload anything or access wiki credentials.
For release documentation, build from a clean checkout of the release tag.
See [local website development](../DEVELOPMENT.md#command-manual-website) for
live reload, service metadata and logo caching.

## Publish on Toolforge

From a clean checkout of the release tag, with Toolforge SSH access configured:

```sh
make manual-install
make manual-check
make manual
./bin/mw toolforge cp --tool a-wikimedia-cli website/dist/ public_html/
```

Check https://a-wikimedia-cli.toolforge.org/ after uploading. No restart is
needed for content updates; the copy retains stale remote files.

First-time server setup only:

```sh
./bin/mw toolforge shell --tool a-wikimedia-cli -- sh -s < website/scripts/setup-toolforge.sh
```

To apply server configuration changes to an existing deployment:

```sh
./bin/mw toolforge shell --tool a-wikimedia-cli -- sh -s -- restart < website/scripts/setup-toolforge.sh
```

The script downloads Caddy and runs it in Toolforge's `trixie` image, with a
custom 404 page. Toolforge handles HTTPS; `robots.txt` allows search indexing.

## Sources and links

Edit command help in its command package. Add user guides under `guides/`, list
their source paths in `guides.json`, and link them from `index.md`. The manifest's
`site_url` supplies the online guide destination. Optional `legacy_url` entries
rewrite old wiki links in the website; they are not publication destinations.
Relative Markdown links between guides are resolved locally and checked during
the site build.

Imported guides retain their revision-specific MediaWiki.org attribution links.
The guides are available under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/);
see the linked page histories for contributors. The generated reference remains
derived from command source under the repository license.

## Retire old wiki references

```sh
make manual-retirement-plan
```

This generates `_docs/reference-retirement.json` by reading the public
`Cli/ref` page list and mapping it to the generated manual. It does not log in,
edit, redirect or delete pages. Entries include the target page, how it was
matched, a proposed moved notice and a deletion reason. Alias mappings and
retired commands resolving only to an ancestor need review. Translations and
unrecognized titles are flagged separately; the plan does not establish that a
page was bot-generated or that its contents can be discarded.

Publish and verify the replacement site before applying a reviewed retirement
plan. Keep manually maintained project pages, guides and their attribution
histories outside this reference-only pass.

## Check workflow examples

Run `make manual-check` and `make manual` to validate documentation links and
`make linti` to check command metadata. Runtime examples are covered by
`tests/guide-examples.sh` in the existing Docker integration jobs. Use a separate
development context for manual checks; the full integration scripts create and
destroy their test environment.

## Record a guide

Keep a [VHS](https://github.com/charmbracelet/vhs) `.tape` beside its guide, with the same basename (for example `guides/tools/kubernetes-pod-helloworld.md` and `guides/tools/kubernetes-pod-helloworld.tape`). Keep any recording setup in a matching `.record.sh` script beside them. Commit these sources; generated media stays in the ignored `_recordings/` directory.

```sh
TOOL=your-tool ./docs/guides/tools/kubernetes-pod-helloworld.record.sh
```

Install mise and FFmpeg first; mise provides pinned VHS and ttyd versions. Toolforge SSH must work without interactive prompts. If VHS stalls before typing, try a different Chromium executable or launcher named `chrome` at the front of `PATH`.

Keep recording-only setup inside `Hide`/`Show`. Select a disposable context by setting `CONTEXT` in the hidden shell setup, rather than adding recording context flags to visible commands. The recording scripts clean shell harness lines from VHS text exports with `tools/clean-recording-text.py`; run it manually for recordings made directly with VHS.

This recording extracts its manifest from the guide, creates and deletes only `cli-guide-hello`, and forwards localhost port 18080. It refuses to start if that pod exists. After an interrupted recording, stop its port forward and remove only the demo pod before retrying.

Add or update the guide/command mapping in `docs/recordings.json`. Site generation
copies available videos and optional matching `.webp` posters into ignored
`website/public/media/`. Missing recordings are omitted; previously staged media
is retained. Keep generated recordings out of Git.

## Record local development workflows

The local development recordings use a running Docker Compose environment and need no Wikimedia account:

```sh
CONTEXT=recording-setup ./docs/guides/dev/fresh.record.sh
CONTEXT=recording-setup ./docs/guides/dev/api.record.sh
CONTEXT=recording-setup ./docs/guides/dev/mysql-dumps.record.sh
./docs/guides/analytics/pageviews.record.sh
./docs/guides/gitlab/artifacts.record.sh
./docs/guides/updating.record.sh
CONTEXT=recording-setup-final ./docs/guides/dev/lifecycle.record.sh
CONTEXT=recording-clean ./docs/guides/dev/extension-workflow.record.sh
CONTEXT=recording-setup-final ./docs/guides/dev/tunnel.record.sh
```

The Fresh recording also requires `mw dev --context recording-setup mediawiki fresh npm ci` first. To record the initial setup flow, point it at a disposable MediaWiki checkout; the script creates a fresh context, installs the wiki and leaves its services running:

```sh
MEDIAWIKI_CODE=/path/to/disposable-mediawiki CONTEXT=recording-setup \
  ./docs/guides/dev/setup.record.sh
```

The setup script refuses to reuse a context that already has Docker containers or volumes, so choose a new context for each fresh setup. All generated media and the temporary database dump go in the ignored repository-root `_recordings/` directory.

The extension workflow recording needs an installed disposable wiki checkout without Cite already cloned or enabled; set `CONTEXT` to that context. The tunnel recording starts a temporary authenticated Cloudflare Quick Tunnel and stops it on exit, including when recording fails.
