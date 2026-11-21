# Choose a development context

A context has its own configuration, Compose project, containers and volumes. Use contexts for separate environments, such as two MediaWiki checkouts. This is separate from a Docker context, which selects the Docker daemon.

## Select a context for one command

```sh
wm dev --context work mediawiki create
wm dev --context work compose ps
wm dev --context work where
```

The first command runs the usual setup for the `work` context. Choose a different HTTP port if another environment is already using the default port; see [ports](ports.md).

## Select a context for a shell

```sh
export CONTEXT=work
wm dev compose ps
wm dev --context other compose ps
unset CONTEXT
```

An explicit `--context` overrides `CONTEXT`. With neither set, the CLI uses `default`. `CONTEXT` is a shell environment variable, not a persisted `wm config` setting.

## Keep configuration separate

Use the same context when changing settings and starting services:

```sh
wm dev --context work env set PORT 8088
wm dev --context work mediawiki create
```

Contexts do not copy source code. If two contexts mount the same checkout, they share files such as `LocalSettings.php`, extensions and uploaded files stored in that checkout. Choose separate checkouts when you need independent source or settings.

To select a different Docker daemon, use Docker's own `docker context use` or `DOCKER_CONTEXT`. That setting and the CLI's `--context` can be used together.
