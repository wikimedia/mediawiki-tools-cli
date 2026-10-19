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

Help-size checks warn when `Long` exceeds 450 words, or `Example` exceeds 12 nonblank lines or 150 words. Word counts do not depend on manual line wrapping. Keep the command's purpose, important behavior, and safety details in help; put step-by-step workflows and recovery recipes in `docs/guides/` and link to them.

These are warnings so existing long help can be shortened incrementally. `make docs` checks guide links and generates the command reference; integration tests validate the workflows that need running services.

Commands can opt out using `mwcli-lint-skip`, or skip embedded third-party command trees with `mwcli-lint-skip-children`:

```go
cmd.Annotations["mwcli-lint-skip-children"] = "Embedded third-party CLI"
```
