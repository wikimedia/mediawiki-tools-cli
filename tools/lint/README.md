# Repository lint

Run `make linti` from the repository root to check command metadata and examples.

The command checks that executable leaf commands with inputs have `Long` help,
that positional arguments shown in `Use` have a Cobra `Args` validator, and that
command names and aliases do not collide among siblings. It also checks that
flag names use lowercase hyphen-separated words and that each flag has help
text. Missing long help and unconventional flag names are suggestions; missing
argument validation and flag descriptions are warnings; name collisions are
errors. Commands imported from another CLI can use the lint skip annotations
when their help and tests are maintained upstream.

Help-size checks suggest a readability review when `Long` exceeds 450 words,
or `Example` exceeds 12 nonblank lines or 150 words. These are review thresholds,
not limits: complex commands can need longer help and more examples. Word counts
do not depend on manual line wrapping. Keep command behavior, defaults, side
effects, safety details and usage examples in help. Move broader workflows to
guides when that improves the explanation; do not shorten help just to clear a
suggestion. See [Writing command help and guides](../../docs/writing.md).

`make manual-check` and `make manual` validate guide links and build the site; integration
tests validate the workflows that need running services.

Commands can opt out using `mwcli-lint-skip`, or skip embedded third-party command trees with `mwcli-lint-skip-children`:

```go
cmd.Annotations["mwcli-lint-skip-children"] = "Embedded third-party CLI"
```

## Command capability hints

`make linti` reports classified/total executable commands and a coverage percentage.
Every executable command missing hints produces a warning, including hidden
commands, executable parents, Cobra's built-in help/completion commands and
imported command trees. Aliases count once.
The lint skip annotations above do not exclude commands from hint coverage.
Non-executable grouping/help-topic commands do not need hints. Invalid or partial
hints are errors and do not count as classified.

Set `Annotations: commandmeta.Annotations(commandmeta.Hints{...})` in the
command's Cobra struct literal so its capabilities stay beside its implementation.
For commands returned by shared constructors, use `commandmeta.Set` at their
registration site to attach hints while preserving existing annotations.
All four behavioral fields use MCP names: `readOnlyHint`,
`destructiveHint`, `idempotentHint` and `openWorldHint`. Two additional fields are
mw-specific:

- `dataAccess` is a list of data categories retrieved or displayed:
  `public`, `potentially-private-content`, `potentially-private-metadata`,
  `account-metadata`, `supplied-content` or `secrets`. Use `none` or `unknown`
  alone when appropriate. Empty, duplicate or unrecognized categories are errors.
- `secretInput` identifies dedicated credential input/import, not ordinary use of
  saved credentials. It does not mean credentials are printed. Arbitrary APIs
  with `dataAccess: ["unknown"]` can accept or expose secrets depending on input.

Potentially private means the command can access restricted objects, not that
every result is private. Account metadata means profile names, endpoints and
usernames, not task/paste content or unmasked credentials. Supplied content means
caller-provided text that the command previews, rather than existing remote
content. Secrets means unmasked credential data retrieved for the caller.
Authentication alone does not classify a command as private content or secrets.
A read-only command is not necessarily public.
Destructive includes overwriting existing fields, not just deleting objects.
Idempotence concerns repeated effects, not whether query results stay identical.

Hints describe the conservative capabilities of the whole command, not a grant
of permission or an argument-specific guarantee. For example, `--dry-run` does
not reclassify a write command, and `--public-only` does not reclassify a private
read. Routine internal caching is not considered a domain mutation. Hints are
not inherited from parent commands. Phabricator is the first classified family;
its existing task visibility checks remain separate and unchanged. Future MCP
integration can expose the four standard fields separately from mw metadata.
