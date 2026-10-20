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

`make docs` checks guide links and generates the command reference; integration
tests validate the workflows that need running services.

Commands can opt out using `mwcli-lint-skip`, or skip embedded third-party command trees with `mwcli-lint-skip-children`:

```go
cmd.Annotations["mwcli-lint-skip-children"] = "Embedded third-party CLI"
```
