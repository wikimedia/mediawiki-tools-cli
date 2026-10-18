<!-- Imported from https://www.mediawiki.org/wiki/Cli/FAQ?oldid=8232967 (CC BY-SA 4.0); see the page history for contributors. -->

# Troubleshoot a development environment

## Choose another MediaWiki checkout

Clear the saved code directory to let the setup wizard ask for it again:

```sh
mw dev env delete MEDIAWIKI_VOLUMES_CODE
mw dev mediawiki create
```

`mw dev env clear` clears all context variables instead. Use it only when you want to configure the entire context again. Neither command is a substitute for destroying service containers or volumes.

To keep another checkout as a separate environment, use a different context:

```sh
mw dev --context another mediawiki create
```

## Docker reports missing cgroup support

Messages such as `Your kernel does not support cgroup memory limit` or `Unable to find cpu cgroup in mounts` come from the Docker host setup. Check the installed Docker version and host configuration:

```sh
docker version
docker info
```

Updating an old Docker installation may resolve these warnings. When Docker runs inside a VM, check the Docker installation and cgroup support on that VM.

See [setup](setup.md), [service configuration](services.md) and [MediaWiki debugging](mediawiki.md) for related workflows.
