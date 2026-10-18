# Repository lint

Run `make linti` from the repository root to check command metadata and examples.

Help-size checks warn when `Long` exceeds 200 words, or `Example` exceeds 12 nonblank lines or 150 words. Word counts do not depend on manual line wrapping. Keep the command's immediate purpose and a few copyable examples in help; put full workflows in `docs/guides/` and link to them.

These are warnings so existing long help can be shortened incrementally. `make docs` checks guide links and generates the command reference; integration tests validate the workflows that need running services.

Commands can opt out using `mwcli-lint-skip`, or skip embedded third-party command trees with `mwcli-lint-skip-children`:

```go
cmd.Annotations["mwcli-lint-skip-children"] = "Embedded third-party CLI"
```
