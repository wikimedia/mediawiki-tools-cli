# Build custom service images

Use a custom Dockerfile to add packages or change a service image. The commands below use MediaWiki; replace `mediawiki` with another service as needed. Use `--context NAME` for a separate environment.

## Create and edit a Dockerfile

```sh
mw dev mediawiki image dockerfile set
mw dev mediawiki image dockerfile edit
```

Without a path, `set` creates or reuses `Dockerfile.mediawiki` in the environment directory. The starter includes the bundled base image and commented package-installation examples. `edit` opens it with `$VISUAL`, `$EDITOR`, or `vi`.

To use an existing Dockerfile instead:

```sh
mw dev mediawiki image dockerfile set ./docker/mediawiki/Dockerfile
```

Relative paths are resolved from your current directory. Keep the base image compatible with the service's runtime and mounts; the CLI generates the Compose build override for you.

## Build and apply changes

```sh
mw dev mediawiki rebuild
```

`rebuild` builds the image and recreates the service containers. It preserves their volumes and reuses Docker's build cache. It is equivalent to:

```sh
mw dev mediawiki create --build --force-recreate
```

Repeat `rebuild` after editing the Dockerfile or files it copies. An ordinary `create` can reuse an existing image and does not guarantee a rebuild. Services using a pre-built image are recreated without a Dockerfile build.

## Understand the build context

The directory containing the Dockerfile is the build context. `COPY` and `ADD` can only use local files inside that directory. Put a `.dockerignore` alongside the Dockerfile to exclude credentials, caches and unrelated files from the build context.

A generated starter lives in the environment directory, so that directory is its context. For a build that copies application files, keep an existing Dockerfile alongside those files and activate it with `set PATH`.

## Inspect the active Dockerfile

```sh
mw dev mediawiki image dockerfile get
mw dev mediawiki image dockerfile cat
```

`get` prints the configured path; `cat` prints its contents. `image get` shows the service's configured image name, and `mw dev compose config` shows the combined Compose configuration.

## Restore the bundled image

```sh
mw dev mediawiki image dockerfile reset
mw dev mediawiki create --force-recreate
```

Reset removes the build override but keeps the Dockerfile on disk. Recreating replaces the running containers while preserving their volumes.

A pre-built image selected with `image set` is a separate override. If one is set, clear it before recreating to return to the bundled image:

```sh
mw dev mediawiki image reset
mw dev mediawiki create --force-recreate
```

See [service management](services.md) for lifecycle and image commands.
