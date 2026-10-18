# Documentation publishing

[Read the guides](index.md). Markdown guides live in `guides/`; `wiki.json` maps each source to its MediaWiki.org title. The command reference comes from Cobra help. Both are built from the release checkout and published together after a release. Existing `Cli/` URLs remain stable during the rebrand; the reference index follows the default executable name.

## Build and review

Install Pandoc and Go, then run:

```sh
make docs
make docs-check
```

`_docs/` contains Markdown, converted wikitext and `pages.json`. `docs-check` reads the managed wiki pages without credentials, prints proposed updates and flags community edits as `WIKI-EDIT`. `_docs/wiki/` contains a `report.md` index, current wikitext, diffs and a revision snapshot. CI keeps these files as documentation job artifacts.

Published pages carry a content fingerprint. If someone edits a page on the wiki, the next publish stops before making any edits. For the initial migration, only reference content verified against the known legacy bot’s publication is adopted automatically (including identical human reverts); imported guides have explicit revision/content baselines in `wiki.json`.

## Bring wiki corrections into Git

```sh
make docs-pull
```

This saves Markdown proposals for guides alongside the wikitext and diffs. It does not modify tracked sources. Merge relevant guide corrections into `docs/`, or update the command help for reference corrections. Check the conversion: wiki templates and other wiki-specific markup may need manual adjustment.

After reviewing a flagged page, acknowledge the exact downloaded content:

```sh
make docs-accept page="Cli/ref/mw_docker"
```

Commit the source fixes and updated `wiki.json`. Acceptance is tied to that snapshot; a later wiki edit is flagged again. Re-run `make docs-check` before tagging a release.

## Add or import a guide

For a new guide, add Markdown under `guides/`, a `source`/`title` entry in `wiki.json`, and a link in `index.md`. Relative Markdown links between mapped guides are converted to wiki URLs. Broken guide links fail the build.

To adopt an existing wiki page:

```sh
make docs-import page="Cli/guide/Gitlab" file="guides/gitlab.md"
```

This imports the current revision into Markdown, adds its mapping and records its baseline and attribution. Importing over an existing source saves a `.bak` copy first. Review the conversion and commit the imported files. Unmapped community pages are never published, deleted or renamed by this tool.

## Publish

Set `DOCS_UPDATE_USER` and `DOCS_UPDATE_PASS` to the wiki publishing account and bot password, then run:

```sh
make VERSION=vX.Y.Z docs-publish
```

The release job uses the existing CI secrets. Credentials are read from the environment. Publishing repeats the drift check and sends the inspected revision ID with every edit; a concurrent wiki edit, deletion or creation causes an API conflict instead of an overwrite. There is no force-publish bypass. A conflict during publishing can leave earlier pages updated; correct it and rerun to finish.

The guides are available under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Imported pages retain revision links for attribution to their wiki contributors. Generated reference remains derived from the command source under the repository license.
