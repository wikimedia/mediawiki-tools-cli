# Spec: `mw translatewiki add` — onboard a gerrit-hosted MediaWiki skin/extension

This spec defines a new `mw` command that automates the process described in
[Add a gerrit-hosted MediaWiki skin or extension to translatewiki.net](../guides/wiki/translatewiki-gerrit-onboarding.md).
It turns that manual workflow into a reproducible command that runs the two
pre-checks, writes the configuration entry, validates it, and prepares a Gerrit
change with a best-practices commit message.

This is an implementation specification for contributors to the CLI, not user
documentation. User-facing behavior lives in the command's own `--help` once
built (see [writing.md](../writing.md)).

## Goal

Given a gerrit-hosted MediaWiki extension or skin, produce a single, review-ready
change to the translatewiki.net configuration repository that registers it for
translation, having first verified that:

1. every source message is documented, and
2. messages that should be `optional` or `ignored` are classified from their
   `qqq.json` documentation (`{{optional}}` / `{{notranslate}}`) and written into
   the configuration entry.

The command does the mechanical work and the analysis; a human reviews the
proposed diff and the Gerrit change before it is submitted.

## Command surface

Place the feature under a new top-level command `translatewiki` (alias `twn`),
registered in `cmd/root.go` alongside the other service commands, in a package
`commands/translatewiki/`. This mirrors `commands/codesearch` and
`commands/gerrit` in structure.

```text
mw translatewiki add <project> [flags]
```

Subcommands:

- `mw translatewiki add` — the end-to-end onboarding flow (primary command).
- `mw translatewiki check <project>` — run only the pre-checks and print a
  report; make no changes. (The `add` flow calls the same logic.)

`<project>` identifies the Gerrit project, accepted as either:

- a short name with type, e.g. `--type extension Foo` / `--type skin Bar`, or
- a full Gerrit project path, e.g. `mediawiki/extensions/Foo` or
  `mediawiki/skins/Bar`, from which the type and name are inferred.

### Flags

| Flag            | Default    | Purpose                                                                                                                                                                        |
|-----------------|------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--type`        | inferred   | `extension` or `skin`. Required if not inferable from the project path.                                                                                                        |
| `--source-host` | `gerrit`   | Where the project's source is hosted. Only `gerrit` is accepted for now; any other value is rejected with an error. Reserved for future hosts (e.g. GitHub, Codeberg, custom). |
| `--config-repo` | discovered | Path to the local `translatewiki` config repo checkout. Discovery order: this flag, `$MWCLI_TWN_CONFIG_REPO`, a `translatewiki` sibling of the current directory, then prompt. |
| `--source`      | fetched    | Path to a local checkout of the skin/extension. If absent, the command reads `i18n/en.json` and `i18n/qqq.json` from the `--source-host` over HTTPS instead of cloning.        |
| `--branch`      | `master`   | Source branch to read i18n from.                                                                                                                                               |
| `--label`       | derived    | Human-readable group label for the entry. Defaults to the project name split into words.                                                                                       |
| `--id`          | derived    | Explicit message-group id. Needed for multi-group projects.                                                                                                                    |
| `--dry-run`     | `false`    | Run pre-checks and print the proposed entry and diff without writing files.                                                                                                    |
| `--no-review`   | `false`    | Write and commit the change but do not run `git review`; leave it for manual submission.                                                                                       |
| `--bug`         | none       | Phabricator task id (e.g. `T12345`) added as a `Bug:` trailer.                                                                                                                 |
| `--yes`         | `false`    | Skip interactive confirmation of the classification and diff (honours the global `--no-interaction`).                                                                          |

## Behavior

The `add` command runs these phases in order. Any phase failing aborts before
changes are committed; already-written working-tree edits are reported so the
user can discard them.

### Phase 0 — Resolve inputs

- Validate `--source-host`. Accept only `gerrit` (the default); reject any other
  value with a clear error that names the supported value, so adding more hosts
  later is a non-breaking extension. All source resolution below is specific to
  the Gerrit host while `gerrit` is the only accepted value.
- Determine project type and name, and compute the Gerrit project path and the
  derived repong repo name (project name with spaces removed, or the leading
  segment of a `file` path for multi-group projects).
- Locate the config repo (`--config-repo` discovery order above) and confirm it
  is a clean checkout of the `translatewiki` project on `master`
  (read `.gitreview`; refuse to proceed on a dirty tree unless `--dry-run`).
- Acquire the source i18n: use `--source` if given, else fetch `en.json` and
  `qqq.json` for the branch from the `--source-host` (Gerrit over HTTPS).
  Reuse the CLI HTTP client (`cli.NewHTTPClient()`), which already sets the
  standard User-Agent.

### Phase 1 — Pre-check 1: all messages documented

This mirrors MediaWiki's standard `grunt-banana-checker` (the `banana` task):
every `en.json` key must be documented in `qqq.json`. The command may either
reimplement the key-subset check directly (no Node.js dependency) or shell out to
the banana checker via a Node-capable path (e.g. `mw dev mediawiki npx --
banana-checker <i18n-dir>`); the direct check is preferred so the command has no
runtime dependency on a dev-environment container.

- Parse `en.json` and `qqq.json`, drop `@metadata`.
- Compute `keys(en) \ keys(qqq)`.
- If non-empty: fail with a clear report listing each undocumented key, and state
  that documentation must be added **upstream** in the project's own repository,
  not in the config repo. Exit non-zero (this is a hard gate).
- `mw translatewiki check` reports the same but always exits after reporting.

### Phase 2 — Pre-check 2: classify optional/ignored messages

Classify each key into `normal` (default, no entry), `optional`, or `ignored`,
then let the user confirm. The categories follow
[Help:System message](https://www.mediawiki.org/wiki/Help:System_message#Messages_that_should_not_be_translated):
`ignored` messages exist only in English because they reference only other
messages or language-neutral features (e.g. `{{SITENAME}}`); `optional` messages
are translated only if changed in the target language. Everything else is
`normal`.

Primary signal — the `qqq.json` documentation template. This is authoritative
and must drive the default classification:

- qqq contains `{{notranslate}}` → **ignored**.
- qqq contains `{{optional}}` → **optional**.
- neither → **normal**.

The classifier must not infer `ignored`/`optional` from the English content alone (CSS/JS page names, separators, URLs,
etc. are frequently still translated in
some languages). Content/key patterns may only be used to **surface suggestions**
for the user to review — never to silently downgrade a message. Any such
suggestion is presented as "consider optional/ignored; qqq has no
`{{optional}}`/`{{notranslate}}` template" so the user can decide, and the
recommended remedy is usually to add the template upstream rather than to list
the key in the config without documentation.

The classifier is deliberately lenient and biased toward translation: when the
qqq template is absent, default to `normal`, and when a suggestion is ambiguous,
prefer `optional` over `ignored` (ignored removes the message from translation
entirely). It must never silently hide a translatable message.

Output of this phase is a proposed classification table, with the qqq-template
basis shown per key and template-less suggestions clearly marked. Unless `--yes`
/ `--no-interaction`, present it for review and allow the user to move keys
between `normal` / `optional` / `ignored` before continuing. Record counts and
the basis (qqq template vs. reviewed suggestion) per key for the commit message.

Core vs. extensions: this command targets extensions and skins, where both
categories are listed in the `.txt` entry. It does not handle the different core
mechanism (optional keys under `optional:` in `MediaWiki.yaml`; ignored keys
moved to `languages/i18n/nontranslatable/`).

### Phase 3 — Compose the configuration entry

Build a `.txt` entry in the grammar used by
`groups/MediaWiki/mediawiki-extensions.txt` / `mediawiki-skins.txt`:

- First line: the label.
- Add `id` / `file` only when the layout is non-default or the project exposes
  multiple groups; add `aliasfile` / `magicfile` / `descmsg` when the source
  tree contains the corresponding files (detect from the checkout or Gerrit
  listing).
- Emit `optional =` / `ignored =` lines from Phase 2, keeping each line within a
  recommended 120 characters and wrapping longer lists across repeated keys to
  match existing formatting.

Insert the entry in alphabetical order by label into the correct file. Do not
hand-edit `repoconfig.yaml`: gerrit extension/skin repository management is
generated by `groups/MediaWiki/repong-generator.php`.

### Phase 4 — Validate

Run the config repo's own checks and fail on any error, surfacing their output:

```sh
php groups/MediaWiki/repong-generator.php <extensions|skins>
php validate-repoconfig.php
php validate-yaml.php
```

Confirm the new project appears in the generator output with the expected
`wmgerrit` URLs:

```text
https://gerrit.wikimedia.org/r/mediawiki/<type>/<Name>
ssh://l10n-bot@gerrit.wikimedia.org:29418/mediawiki/<type>/<Name>
```

If `php` is unavailable, skip with a clear warning and mark the result as
"unvalidated" rather than failing (the CI on the config repo will still run the
checks). Document this fallback in help.

### Phase 5 — Commit and submit

- Stage only the edited `.txt` file.
- Create a commit using the best-practices template below (built from Phase 1/2
  results). Let Gerrit's `commit-msg` hook add the `Change-Id`.
- Unless `--no-review`, run `git review` to push to `refs/for/master` and print
  the resulting change URL. With `--no-review`, stop after the commit and print
  the next command.

Commit message template the command generates:

```text
Add <Label> for translation

Register the gerrit-hosted <extension|skin> <Name> as a translatable
project. All messages in i18n/qqq.json are documented.

Classified <N> message(s) as optional and <M> as ignored, following their
i18n/qqq.json documentation ({{optional}} / {{notranslate}}).

Bug: <T-id>        # only when --bug is given
```

Subject under ~72 characters, imperative mood, body wrapped at ~72. Omit the
`Bug:` trailer when no task is supplied.

## Architecture and reuse

- New package `commands/translatewiki/` with `translatewiki.go` (command tree),
  `add.go`, `check.go`, a `classify.go` holding the heuristics (unit-tested in
  isolation), and an embedded `translatewiki.long.md` plus `.example` files.
- Reuse existing internals rather than reimplementing:
    - `cli.NewHTTPClient()` for Gerrit HTTPS reads (consistent User-Agent).
    - The pattern in `commands/gerrit` for anything needing authenticated Gerrit
      access; `git review` is invoked as a subprocess in the config repo, matching
      how other commands shell out.
    - `internal/util/...` helpers for running subprocesses and prompting, honoring
      `--no-interaction`.
- The `.txt` parser/writer must round-trip existing files without reformatting
  unrelated entries: parse into entries, insert, and re-emit only with minimal
  diff (preserve comments and blank-line separation).

## Testing

- Unit tests for `classify.go` using fixtures of `en.json`/`qqq.json` pairs that
  cover: qqq with `{{notranslate}}` → ignored, qqq with `{{optional}}` →
  optional, qqq with neither → normal, and content-pattern cases (CSS/JS names,
  separators, URLs) that have no template → normal with a review suggestion, not
  an automatic ignored/optional.
- Unit tests for the `.txt` insert/emit that assert alphabetical placement and a
  minimal diff against a golden file.
- A dry-run end-to-end test with a fixture config repo and a fixture source tree
  that asserts the proposed entry, the classification report, and the generated
  commit message — without invoking `php` or `git review`.
- Guard the `php`/`git review` steps behind interface seams so tests can stub
  them; real invocations are covered by an optional integration test gated like
  the existing docker integration tests.

## Non-goals

- No change to how gerrit repong config is generated; the command relies on the
  existing generator.
- No upstream editing: if Pre-check 1 fails, the command reports and stops; it
  does not write `qqq.json` in the source project.
- No translator-rights, wiki-side group descriptions, or announcements.
- GitHub/GitLab-hosted projects are out of scope for this command (different
  repong types and list files).

## Acceptance criteria

- `mw translatewiki check <project>` reports documentation completeness and a
  proposed optional/ignored classification, and exits non-zero when messages are
  undocumented.
- `mw translatewiki add <project> --dry-run` prints the proposed `.txt` entry,
  the classification, and the generated commit message, and changes nothing.
- `mw translatewiki add <project>` writes the entry alphabetically to the correct
  list, passes the config repo's validators, commits with the templated message,
  and (unless `--no-review`) submits to Gerrit on `master`, printing the change
  URL.
- The classifier never downgrades a translatable message to hidden without
  evidence; ambiguous keys are surfaced for confirmation.
- Behavior, flags, defaults, failure modes, and the `php`-unavailable fallback
  are documented in the command's `--help`, with a `mw help guides` link to the
  onboarding guide.
