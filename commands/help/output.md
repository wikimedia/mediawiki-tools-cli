# Output formats

Many commands that return data offer `--output` to choose a format. Available formats and defaults depend on the command; check its `--help`.

- `table`: Aligned columns.
- `pretty`: Human-readable sections.
- `json`: JSON, indented and coloured in a terminal and compact when piped.
- `jq`: Filter or transform data with a jq expression; string results print without quotes.

Some commands offer additional formats: analytics supports `csv` and `graph`, and commands with `web` can open a browser. Where the default is `auto`, output adapts to whether it is displayed in a terminal or piped.

## Examples

Show version information as a table or JSON:

```sh
mw version --output table
mw version --output json
```

Print just the version string:

```sh
mw version --output jq --format '.version'
```

Graph a ranking, skipping its first entry:

```sh
mw analytics top-pages --output graph --offset 1 --limit 10
```

## Filtering and formatting

`--format` supplies a jq expression for `json` or `jq` output. JSON output keeps JSON encoding; `jq` prints string results without quotes.

Where available, `--filter` selects matching records before formatting. Its conditions depend on the command's data; check that command's help.
