# lint

mwcli custom command linting

Trying to make the commands have a consistent look and feel, one step at a time.

You can run this work in progress tool as follows:

```sh
go run ./tools/lint/main.go
```

You can also run this from the main makefile

```sh
make linti
```

This will soon be included as a pre commit hook...

## Rules

All rules and conditions can currently be seen in `main.go`.

## Skipping

Skipping can be done by adding annotations to commands

### Skip children

You can skip linting certain commands children.

```go
cmd.Annotations["mwcli-lint-skip-children"] = "yarhar"
```
