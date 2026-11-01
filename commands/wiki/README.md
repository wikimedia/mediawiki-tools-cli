# Wiki resources and explicit edits

All commands reuse `mw wiki --wiki SITE --profile REALM/NAME`. `--anon` bypasses
saved credentials and cookies while retaining site selection. Examples below
use dry-run for writes; choose your own test wiki and entity IDs before applying.

## Statements, qualifiers and references

Items and properties expose the same commands under `ext wikibase item` and
`ext wikibase property`:

```sh
mw wiki ext wikibase item statement list Q1 --property P31
mw wiki ext wikibase item statement get Q1 STATEMENT_ID
mw wiki ext wikibase item statement add Q1 P31 --datatype wikibase-item --value Q5 --dry-run
mw wiki ext wikibase item statement update Q1 STATEMENT_ID P31 --datatype wikibase-item --value Q6 --dry-run
mw wiki ext wikibase item statement remove Q1 STATEMENT_ID --dry-run
mw wiki ext wikibase item statement qualifier list Q1 STATEMENT_ID
mw wiki ext wikibase item statement qualifier add Q1 STATEMENT_ID P1 --datatype string --value Example --dry-run
mw wiki ext wikibase item statement qualifier remove Q1 STATEMENT_ID QUALIFIER_HASH --dry-run
mw wiki ext wikibase item statement reference list Q1 STATEMENT_ID
mw wiki ext wikibase item statement reference add Q1 STATEMENT_ID --input reference.json --dry-run
mw wiki ext wikibase item statement reference remove Q1 STATEMENT_ID REFERENCE_HASH --dry-run
```

Statement IDs and hashes are opaque values obtained from reads. Membership is
checked in the explicitly selected entity/statement. Every write uses a current
revision snapshot; `--baserevid N` additionally requires that exact revision.
`--summary`, `--bot`, and `--dry-run` match the existing term-edit commands. Dry-run
reads preconditions but fetches no CSRF token and performs no edit.

Typed scalar updates preserve rank, qualifiers, references and unrelated native
fields. `--rank` deliberately changes rank. `--input statement.json` supplies a
**complete statement replacement** for update, so include the qualifiers and
references you want to keep. New statement input must omit `id`; update input may
omit it or include the selected statement's exact ID. Changing a statement's
property requires explicit removal/addition.

Supported typed datatypes:

| Datatype | Input |
|---|---|
| `string`, `external-id`, `commonsMedia` | Nonempty `--value` |
| `url` | HTTP(S) `--value` without embedded credentials |
| `wikibase-item`, `wikibase-property` | Q/P ID via `--value` |
| `quantity` | Signed/unsigned scalar via `--value` (unit `1`), or native value JSON |
| `monolingualtext` | Native value JSON with `text` and `language` |
| `time` | Native signed Wikibase timestamp, precision `0..14`, calendar URI and time metadata |
| `globe-coordinate` | Native latitude/longitude, positive precision and globe URI |

Use `--value-json value.json` (or `-` for stdin) for complex typed values. Examples:

```json
{"amount":"+12","unit":"http://www.wikidata.org/entity/Q11573","lowerBound":"+11","upperBound":"+13"}
```

```json
{"text":"Example","language":"en"}
```

```json
{"time":"+2026-01-02T00:00:00Z","precision":11,"timezone":0,"before":0,"after":0,"calendarmodel":"http://www.wikidata.org/entity/Q1985727"}
```

```json
{"latitude":51.5,"longitude":-0.1,"precision":0.001,"globe":"http://www.wikidata.org/entity/Q2"}
```

`--snaktype somevalue` and `--snaktype novalue` accept no value. The wiki still
checks whether a property actually has the supplied datatype. A dry-run is a
local/precondition check, not proof that the server will permit the edit.

For other native datatypes, supply a full statement/snak JSON file with
`--input ... --raw-json` (the qualifier command accepts a snak). This bypasses
only datatype-specific validation; ownership, revision checks and server
validation still apply. Read commands preserve native JSON for all datatypes.

A reference file contains a native snaks map, for example:

```json
{"snaks":{"P854":[{"property":"P854","snaktype":"value","datatype":"url","datavalue":{"type":"string","value":"https://example.org/source"}}]},"snaks-order":["P854"]}
```

Reference/qualifier add and remove use the dedicated Action API modules, avoiding
implicit merge/delete behavior. Removing one reference does not remove unrelated
references. New reference input accepts `hash`, `snaks`, and `snaks-order`; a hash
from exported input does not turn `add` into an update.

## Page comparisons and file reads

```sh
mw wiki page diff Earth
mw wiki page diff rev:123 rev:124
mw wiki page diff title:123 title:Earth --output json
mw wiki file info Example.jpg --extended
mw wiki file usage Example.jpg --namespace 0 --limit 20 --output titles
```

A single diff selector compares the selected page/revision with its predecessor;
two selectors compare FROM to TO. Bare positive integers mean revision IDs;
`title:` forces a literal title. Default text uses native unified diffs when the
wiki supports them, otherwise readable +/-/context lines. The older-wiki fallback
is **not an applyable patch**. JSON/jq expose native comparison metadata and HTML;
`--output html` emits the native HTML fragment without opening a browser.

File metadata includes size/dimensions, hash, MIME type and URLs. `--extended`
adds raw/common/extended metadata, which can include HTML strings. Asset URLs are
never fetched by these commands. Use a bare filename or an explicitly qualified
file title; qualify filenames containing colons with `File:`. Shared-repository
files work without a local description page. Usage is paginated, deduplicated
and local to the selected wiki; it does not query global/cross-wiki usage or walk
upload history.

## Explicit JSONL bulk edits

```sh
mw wiki bulk apply --input edits.jsonl --dry-run
mw wiki bulk apply --input edits.jsonl --stop-on-error
```

Use `--input -` for stdin. Each nonblank line is one JSON object (maximum 8 MiB).
Unknown fields and unsupported operations fail that record. Every record needs
`operation` and a nonempty `summary`; optional `record_id` is echoed in results.
Existing-resource updates require an explicit positive `baserevid`. No current
revision is substituted for a supplied value. Creates require revision `0`.

| Operation | Additional fields |
|---|---|
| `page-put` | `title`, `text`; new pages also require `createonly:true` |
| `entity-create` | `kind`, native entity `data` without existing ID |
| `term-change` | `kind`, `entity`, `term`, `change`, `language`, `values` |
| `statement-add` | `kind`, `entity`, native `statement` without ID |
| `statement-update` | `kind`, `entity`, `statement_id`, complete native `statement` |
| `statement-remove` | `kind`, `entity`, `statement_id` |
| `qualifier-add` | `kind`, `entity`, `statement_id`, native `snak` |
| `qualifier-remove` | `kind`, `entity`, `statement_id`, `hash` |
| `reference-add` | `kind`, `entity`, `statement_id`, native `reference` |
| `reference-remove` | `kind`, `entity`, `statement_id`, `hash` |

`kind` must be `item` or `property`. Term changes support label/description
set/remove and alias set/add/remove. Statement/qualifier/reference additions or
updates accept `raw_json:true` for datatypes outside the typed subset. Records
may set `bot:true`; the wiki still checks the account's rights.

```jsonl
{"record_id":"label-1","operation":"term-change","kind":"item","entity":"Q1","term":"label","change":"set","language":"en","values":["Example"],"baserevid":123,"summary":"Correct label"}
{"operation":"page-put","title":"User:Example/Sandbox","text":"Example text","createonly":true,"baserevid":0,"summary":"Create sandbox"}
```

Results stream as NDJSON, independently of human table settings:

```jsonl
{"line":1,"record_id":"label-1","operation":"term-change","status":"planned","result":{}}
{"line":2,"operation":"page-put","status":"error","error":{"code":"conflict","message":"Page revision changed"}}
```

Statuses are `planned`, `success`, or `error`. Any failed record causes a failing
final exit status. Processing continues by default; `--stop-on-error` stops after
the first failure. Earlier successes are not rolled back. Repeated edits to the
same entity need fresh explicit revisions; the runner does not silently advance
preconditions for later records.

If a POST loses its response or returns an indeterminate server failure, the
result includes `error.ambiguous:true`. Inspect the wiki before retrying that
record. There are no automatic write retries, including for creates. If result
output fails, processing stops rather than applying further unreportable edits.
There is no transaction across records, script execution, arbitrary API-operation
pass-through, or implicit conversion of selected/read pages into writes.

## Compatibility boundaries

The limited syntax adapters are separate from these canonical resources:
[Pywikibot selectors](compat_pywikibot.go) and
[wikibase-cli term redirects](compat_wikibase_cli.go). Their headers link upstream
interface documentation, list supported subsets and differences, and describe
how to remove them. They are original syntax adapters, not copied upstream
implementations, Python/JavaScript runners, or a separate `wb` executable.

## Sitelinks and statement ranks

```sh
mw wiki ext wikibase item sitelink list Q42
mw wiki ext wikibase item sitelink get Q42 enwiki
mw wiki ext wikibase item sitelink set Q42 enwiki 'Douglas Adams' --dry-run
mw wiki ext wikibase item sitelink set Q42 enwiki 'Douglas Adams' --badge Q17437796 --dry-run
mw wiki ext wikibase item sitelink set Q42 enwiki 'Douglas Adams' --clear-badges --dry-run
mw wiki ext wikibase item sitelink remove Q42 enwiki --dry-run
mw wiki ext wikibase item statement update Q42 STATEMENT_ID --rank preferred --dry-run
```

Sitelinks belong to items. `set` preserves badges unless repeated `--badge` replaces
**all** badges or `--clear-badges` explicitly empties them. The wiki validates its
site IDs and available badges. Removing an absent sitelink fails before writing.
Reads retain native fields, including returned URLs. Mutation flags and revision
preconditions match the other entity commands. Rank-only statement updates accept
`normal`, `preferred`, or `deprecated` and preserve the mainsnak, qualifiers,
references, and extension fields, including native datatypes outside the typed
value subset. These operations are individual edits; the bulk operation whitelist
is unchanged.

## Composable page selections

```sh
mw wiki page list --select category:Physics --select search:gravity --combine intersection --depth 1 --namespace 0 --limit 20
mw wiki page list --select backlinks:Earth --select transclusions:Template:Planet --combine union
mw wiki page list --select titles:pages.txt --select search:planet --combine intersection
```

`--select SOURCE:VALUE` is repeatable; supported sources are `search`, `category`,
`backlinks`, `transclusions`, and `titles` (plain title file, or `-` for stdin).
It can accompany one existing canonical selector, which is evaluated as the first
operand regardless of flag order. Union is the default and keeps
first-seen order; intersection keeps the first operand's order. All operands are
fully paginated before composition: `--limit` never truncates an operand before an
intersection. Namespace, literal grep, deduplication, and the distinct-page limit
apply to the final set. Explicit composition can therefore read considerably more
pages than its final limit. There is no implicit allpages operand; without explicit
selectors the existing default allpages behavior remains unchanged.

`--depth N` follows up to N subcategory edges breadth-first; depth 0 includes only
direct members, and the maximum depth is 20. Category nodes are retained as results
unless namespace filtering excludes them. Traversal tracks visited categories and
pages to break cycles and avoid duplicates. `--max-categories` (default 1000 per
category operand, including the root) aborts with an error instead of returning a
silently truncated set. Recursion does not follow backlinks or transclusions.
Pywikibot compatibility remains a separate single-source adapter and cannot be
combined with `--select`, `--depth`, or `--combine`. Only one title-file operand may consume stdin.

Completion for these resource commands is offline: fixed-value flags suggest
supported values and unknown entity IDs/titles/hashes do not fall back to filenames.
Raw API completion continues to use its cached installation discovery.
