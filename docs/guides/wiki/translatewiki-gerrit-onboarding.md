# Add a gerrit-hosted MediaWiki skin or extension to translatewiki.net

This spec describes the end-to-end process for registering a MediaWiki skin or
extension whose source lives in Wikimedia Gerrit (`gerrit.wikimedia.org`, e.g.
`mediawiki/extensions/Foo` or `mediawiki/skins/Bar`) with
[translatewiki.net](https://translatewiki.net) for message translation.

The deliverable is a single change submitted for review to the translatewiki.net
configuration repository in Gerrit (project `translatewiki`, branch `master`),
with a best-practices commit message. The local checkout of that repository (for
example at `~/repos/translatewiki`) is referred to below as the **config repo**.

This document is a specification, not an automated tool. Each phase lists the
exact file to touch and the checks to perform so the work is reproducible and
reviewable.

## Scope and assumptions

In scope:

- A skin or extension already hosted in Wikimedia Gerrit.
- Standard MediaWiki JSON i18n layout (`i18n/en.json` + `i18n/qqq.json`), the
  convention this guide validates against.
- Registering exactly one project (which may expose more than one message group,
  for example a user-interface group and an API group).

Out of scope (handled by existing, separate configuration and not repeated here):

- GitHub- or GitLab-hosted projects (`mwgithub.yaml` / `mwgitlab.yaml`).
- Non-MediaWiki projects under `groups/` (those use their own templates).
- Granting translator rights, wiki-side group descriptions, or on-wiki
  announcements.

Prerequisites:

- A local clone of the `translatewiki` Gerrit config repository must be present;
  this is the repository the process operates on. The guide refers to it as the
  config repo and assumes it is checked out somewhere local (for example
  `~/repos/translatewiki`). Its `.gitreview` already points
  at `gerrit.wikimedia.org:29418` project `translatewiki`, default branch
  `master`, so the change can be submitted from the clone with `git review`.
- Commit access, or willingness to submit for review, to the `translatewiki`
  Gerrit project.
- A local clone of the target skin/extension (or read access to its `i18n/`
  directory in Gerrit) to run the pre-checks.

## How registration works

Registering a gerrit-hosted MediaWiki skin or extension touches two concerns,
both driven from one place:

1. **Translation group definition** — which messages are offered for
   translation, and how they are validated. For MediaWiki extensions and skins
   this is a single line-oriented entry in one of:
    - `groups/MediaWiki/mediawiki-extensions.txt` (extensions), or
    - `groups/MediaWiki/mediawiki-skins.txt` (skins).

   These `.txt` files are expanded into Translate message-group definitions. The
   aggregate groups `ext-0-all` (extensions) and `mediawiki-skin-0-all` (skins)
   pick the entries up by id pattern, so no aggregate file needs editing for a
   normal addition.

2. **Repository management (repong)** — how translatewiki.net pulls source and
   pushes translations back. For gerrit-hosted extensions and skins this is **generated automatically** from the same
   `.txt` files by
   `groups/MediaWiki/repong-generator.php`, which emits `wmgerrit` repository
   entries of the form:

   ```text
   type:       wmgerrit
   url:        https://gerrit.wikimedia.org/r/mediawiki/<extensions|skins>/<Name>
   url|export: ssh://l10n-bot@gerrit.wikimedia.org:29418/mediawiki/<extensions|skins>/<Name>
   ```

   Because this is generated, a standard gerrit extension/skin addition does **not** require a manual edit to
   `repoconfig.yaml`. The repository name is
   derived from the entry: normally the project name with spaces removed, or the
   first path segment of the entry's `file =` directive when present.

The practical consequence: for the common case, the entire change is a few lines
added to one `.txt` file.

## The `.txt` entry grammar

Entries are separated by blank lines. Lines beginning with `#` are comments (used for notes such as why a project is
skipped). The first line of an entry is
the human-readable project label. Subsequent `key = value` lines refine it.

Supported keys observed in the config repo:

| Key         | Purpose                                                                                                                                                                                                                                               |
|-------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| *(label)*   | The entry's first line — not a `key = value` line, hence shown in parentheses. It is the project label, and with no other keys the group id and repo name are derived from it. What you write here matters; see "The label and how it is used" below. |
| `id`        | Explicit message-group id (e.g. `ext-foo-user`). Required when a project exposes multiple groups. By convention extension ids start with `ext-`.                                                                                                      |
| `file`      | Source pattern for the JSON messages, e.g. `Foo/i18n/%CODE%.json`. The first path segment also becomes the repong repo name.                                                                                                                          |
| `aliasfile` | Path to a special-page alias file, e.g. `Foo/Foo.alias.php`.                                                                                                                                                                                          |
| `magicfile` | Path to a magic-word i18n file, e.g. `Foo/Foo.i18n.magic.php`.                                                                                                                                                                                        |
| `descmsg`   | Message key holding the extension description, when it is not the default `<name>-desc`.                                                                                                                                                              |
| `prefix`    | Shared message prefix plus exceptions: `prefix = foo- \| shared-key1, shared-key2`.                                                                                                                                                                   |
| `optional`  | Comma-separated keys that translators may skip (see classification rules). Repeatable; keep each line within a recommended 120 characters and wrap by repeating the key.                                                                              |
| `ignored`   | Comma-separated keys never offered for translation (see classification rules). Repeatable; keep each line within a recommended 120 characters and wrap by repeating the key.                                                                          |

### The label and how it is used

The first line of an entry has no `key =` prefix, which is why the table shows it
as *(label)* rather than a key name. It is not just a display string: for the
common single-group case it determines three things.

- **Display label** — the human-readable name shown in the translatewiki.net UI
  and group lists, exactly as written (spaces and capitalization preserved). For
  example `Universal Language Selector`.
- **Repository name (repong)** — the label with **all spaces removed**, used
  verbatim to build the Gerrit repo path
  `mediawiki/extensions/<label-without-spaces>` (see
  `groups/MediaWiki/repong-generator.php`). This is **case-sensitive** and is not
  normalized: the de-spaced label must match the real Gerrit project name
  exactly. So `Foo Bar` produces repo `FooBar` and must correspond to
  `mediawiki/extensions/FooBar`; writing `foo bar` would wrongly target
  `foobar`. If the extension's repo name differs from its display label, set an
  explicit `file =` instead, whose first path segment then provides the repo
  name.
- **Group id** — when no explicit `id =` is given, the message-group id is
  derived from the label (lower-cased, prefixed, e.g. an extension becomes
  `ext-<label>`). Provide an explicit `id` when the project has multiple groups
  or when you need a specific id.

Because the repo name comes straight from the label, the safest rule is: **write
the label as the extension's `extension.json` / Gerrit name with spaces inserted
between words, matching its capitalization.** For a repo
`mediawiki/extensions/FooBar`, write `Foo Bar`.

### Example — the simplest extension entry

For an extension whose Gerrit repo is `mediawiki/extensions/FooBar`, whose
messages live at the default path `FooBar/i18n/%CODE%.json`, and whose
description message is the default `foobar-desc`, the entire entry is just the
label:

```text
Foo Bar
```

This yields: display label `Foo Bar`, repo `mediawiki/extensions/FooBar` (spaces
removed), and a derived group id (`ext-foobar`). Nothing else is required for a
standard layout.

Multi-group entry (separate user-interface and API message files):

```text
Foo - User interface
id = ext-foo-user
file = Foo/i18n/%CODE%.json
aliasfile = Foo/Foo.alias.php

Foo - API
id = ext-foo-api
file = Foo/i18n/api/%CODE%.json
```

Entries are kept in alphabetical order by label within the file; follow the
existing ordering when inserting.

## Pre-checks (mandatory before submitting)

Run these against the project's `i18n/` directory before writing the config
entry. They are the gate: do not submit until both pass.

### Pre-check 1 — all messages are documented

Every key in `i18n/en.json` must have a corresponding entry in `i18n/qqq.json`
(message documentation). translatewiki.net expects complete documentation; undocumented
messages are a blocker.

Procedure:

1. Enumerate the keys of `en.json` and `qqq.json` (ignore the `@metadata` key in
   both).
2. Compute the set difference. The set of `en.json` keys must be a subset of the
   `qqq.json` keys.
3. If any key is undocumented, stop. Documentation is fixed **upstream in the
   project's own Gerrit repository**, not in the config repo. Report the
   undocumented keys and request/submit the `qqq.json` additions upstream first.

Preferred check — the `banana` message-documentation checker. MediaWiki's
standard tool for this is `grunt-banana-checker` (the `banana` Grunt task wired
into every extension's `Gruntfile.js` and run by `grunt test`). It verifies that
every `en.json` key has a `qqq.json` entry, and reports undocumented keys and
orphaned documentation.

The checker needs Node.js, which the MediaWiki service container does not have,
so run it through `mw dev mediawiki npx`, which executes in a `fresh` container
that provides Node. Paths are relative to the MediaWiki code root (`MEDIAWIKI_VOLUMES_CODE`). For an example extension
`Foo`:

```sh
# Check one extension's message documentation (relative to the code root).
mw dev mediawiki npx -- banana-checker extensions/Foo/i18n/
```

A pass exits zero with no reported problems; undocumented keys are listed and the
command exits non-zero. The first run pulls the `fresh` image, which can take a
few minutes. For a skin, use `skins/Foo/i18n/`; for an extension with extra
message directories (for example an `api/` subdirectory), check each directory
the entry's `file =` patterns point at.

Dependency-free fallback. If a Node-capable container is not available, the same
subset check can be done with PHP, run in the project checkout:

```sh
# Lists en.json keys that are missing from qqq.json. Empty output = pass.
php -r '
  $en = json_decode(file_get_contents("i18n/en.json"), true);
  $qqq = json_decode(file_get_contents("i18n/qqq.json"), true);
  unset($en["@metadata"], $qqq["@metadata"]);
  $missing = array_diff(array_keys($en), array_keys($qqq));
  foreach ($missing as $k) { echo $k, "\n"; }
'
```

Expected result for a pass: no output. Any listed key must be documented upstream
before continuing.

### Pre-check 2 — classify optional and ignored messages

A few messages should not be offered for normal translation. translatewiki.net
recognizes two narrow categories, defined by
[Help:System message](https://www.mediawiki.org/wiki/Help:System_message#Messages_that_should_not_be_translated):

- **`ignored`** — messages that should exist **only** in the English messages
  file. They need no translation because they reference only other messages or
  language-neutral features — for example a message whose value is just
  `{{SITENAME}}`, or one that is purely another message key or a language-neutral
  token. A message whose English value is an **empty string** is also `ignored`,
  because there is nothing to translate. An ignored message is never shown to
  translators.
- **`optional`** — messages that **may be translated only if they are changed in
  the target language**. They are offered to translators but can be left as the
  English source, so most languages leave them untouched.

Two content patterns reliably indicate `optional`:

- A message whose value is **only a `Special:MyLanguage/...` URL or link** (it
  usually stays the same, but a language may point it elsewhere).
- A message whose value is **only variables/parameters** (for example a single
  `$1`, or several parameters with no surrounding text) — there is nothing to
  translate unless a language needs to reorder or adjust them.

These should be made `optional` (and ideally carry `{{optional}}` in `qqq.json`),
not `ignored`, because a language may still legitimately change them.

Everything else is a normal translatable message and needs no entry. Default to
"normal": only classify a message when there is a clear signal, and prefer
`optional` over `ignored` when unsure, because `ignored` removes the message from
translation entirely. These rules are deliberately lenient — do not mark a
message `ignored` merely because its English value happens to contain markup,
punctuation, or a URL; such messages are often still translated in some
languages.

The authoritative signal is the message's own documentation. In `qqq.json`,
maintainers flag these messages with a template:

- `{{notranslate}}` → the message is **ignored**.
- `{{optional}}` → the message is **optional**.

So the classification is primarily a matter of reading `qqq.json` and honoring
those templates, not guessing from the English content.

Procedure:

1. Read each message's `qqq.json` documentation. If it carries `{{notranslate}}`,
   classify the key as `ignored`; if it carries `{{optional}}`, classify it as
   `optional`.
2. For messages without such a template, leave them as normal translatable
   messages. Only consider adding one yourself when a message clearly matches the
   definitions above (English-only / references only other messages or
   language-neutral features for `ignored`; "translate only if changed" for
   `optional`). When a message plausibly needs translation, leave it normal.
3. Cross-reference existing precedent in the config repo for similar message
   families (search `groups/MediaWiki/*.txt` and `MediaWiki.yaml` for analogous
   keys) so borderline cases match established practice. If the upstream `qqq`
   lacks a template but the message clearly belongs in a category, the better fix
   is often to add `{{optional}}`/`{{notranslate}}` upstream rather than only
   listing it in the config.
4. Collect the keys into `optional =` and `ignored =` lines on the entry. Keep
   each line to a recommended maximum of 120 characters; when a list is longer,
   split it across multiple `optional =` / `ignored =` lines (both keys are
   repeatable), mirroring existing entries.

> Note on core vs extensions. This guide covers extensions and skins, where both
> `optional` and `ignored` keys are listed in the `.txt` entry under the
> project's name. In MediaWiki **core** the mechanism differs: optional core
> messages are listed under `optional:` in `groups/MediaWiki/MediaWiki.yaml`,
> while ignored core messages are moved to
> `languages/i18n/nontranslatable/en.json` (with docs in the matching
> `qqq.json`) instead of being listed in the group configuration.

Validator interaction (optional, advanced): the `MediaWikiParameter`,
`MediaWikiPlural`, `MediaWikiTimeList`, and related validators defined in the
`groups/MediaWiki/*.yaml` templates may raise warnings for messages with
optional parameters. For extensions and skins the per-message `optional` tag is
normally sufficient; only touch validator `exclude`/`include` lists if a
specific validator misfires on a documented-optional message.

## Procedure

1. **Update the config repo checkout.**
   In the config repo clone (for example `~/repos/translatewiki`), switch to
   `master` and
   pull the latest, so the change is based on current state and `git review`
   targets the right branch.

2. **Run the pre-checks** (above) against the project's `i18n/` directory and
   resolve any undocumented messages upstream first.

3. **Add the entry** to the correct list, alphabetically by label:
    - extension → `groups/MediaWiki/mediawiki-extensions.txt`
    - skin → `groups/MediaWiki/mediawiki-skins.txt`

   Include `id`/`file`/`aliasfile`/`magicfile`/`descmsg` only when the project
   deviates from defaults, and the `optional`/`ignored` keys determined in
   Pre-check 2.

4. **Validate locally.** From the config repo root, run the repository's own
   checks so the change matches CI expectations:

   ```sh
   # Confirm the repong repository config still generates and validates.
   php groups/MediaWiki/repong-generator.php extensions   # or: skins
   php validate-repoconfig.php
   php validate-yaml.php
   ```

   Confirm the new project appears in the generator output with the expected
   `wmgerrit` URLs, and that validation reports no errors. Do not hand-edit
   `repoconfig.yaml` for a generated gerrit entry.

5. **Review the diff.** The change should be limited to the one `.txt` file (plus, only if the project is unusual, a
   deliberate manual addition). Verify
   alphabetical placement and that `optional`/`ignored` lists are complete and
   justified.

6. **Commit and submit to Gerrit.** Use `git review` (configured by
   `.gitreview`) to push to `refs/for/master` on `gerrit.wikimedia.org`:

   ```sh
   git add groups/MediaWiki/mediawiki-extensions.txt   # or mediawiki-skins.txt
   git commit            # message per the template below
   git review
   ```

## Commit message (best practices)

Follow the Gerrit/Wikimedia conventions: a concise imperative subject under ~72
characters, a blank line, a body explaining what and why, and trailers. Include
a `Bug:` trailer when a Phabricator task tracks the request. Gerrit adds the
`Change-Id` automatically via the `commit-msg` hook.

Template:

```text
Add <Extension or Skin name> for translation

Register the gerrit-hosted <extension|skin> <Name> as a translatable
project. Messages in i18n/qqq.json are complete.

Classified <N> message(s) as optional and <M> as ignored, following their
i18n/qqq.json documentation ({{optional}} / {{notranslate}}); see the entry
for the full list.

Bug: T000000
```

Guidelines for the body:

- State that Pre-check 1 passed (all messages documented).
- Summarize the Pre-check 2 outcome: how many keys were marked `optional` /
  `ignored`, noting that the classification follows the `{{optional}}` /
  `{{notranslate}}` documentation, so reviewers can sanity-check it without
  re-deriving it.
- Keep lines wrapped at ~72 characters.
- Omit the `Bug:` trailer if there is no tracking task; do not invent one.

## Acceptance criteria

- Pre-check 1 passes: every `en.json` key is documented in `qqq.json`.
- Pre-check 2 is applied: optional/ignored keys follow their `qqq.json`
  documentation (`{{optional}}` / `{{notranslate}}`) and are present in the
  entry, consistent with existing precedent; messages are not marked `ignored`
  without clear justification.
- The entry is placed alphabetically in the correct `.txt` file with the right
  `id`/`file` directives for its layout.
- `php groups/MediaWiki/repong-generator.php <type>`, `php
  validate-repoconfig.php`, and `php validate-yaml.php` all succeed, and the new
  project shows the expected `wmgerrit` URLs.
- A single change is submitted to the `translatewiki` Gerrit project on `master`
  with a commit message following the template above.
