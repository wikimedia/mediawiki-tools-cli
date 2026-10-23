# Debug PHP with Xdebug

The bundled PHP image has Xdebug enabled in `debug` mode and uses `client_host=xdebug://gateway`. The usual debugger port is 9003. Profiling is not enabled by default.

## Connect your IDE

Configure your IDE to listen for Xdebug and map `/var/www/html/w` to your MediaWiki checkout. Xdebug connects from the PHP container to the IDE; the IDE must be reachable at its configured host and port.

`xdebug://gateway` uses the container's gateway on Linux. Docker Desktop, rootless Docker or a remote VM may need another reachable address. If your runtime provides `host.docker.internal`, for example:

```sh
mw dev env set MEDIAWIKI_XDEBUG_CONFIG 'client_host=host.docker.internal client_port=9003'
mw dev mediawiki create --force-recreate
```

Use an address appropriate to your setup. A public HTTP tunnel does not carry Xdebug's separate TCP connection; a remote environment needs suitable private connectivity to your IDE.

## Start a debug session

Use your IDE's browser integration to set a debugging cookie, or add `?XDEBUG_SESSION=1` to a wiki request. Debug mode enables support; a trigger starts the session. See [Xdebug's step-debugging documentation](https://xdebug.org/docs/step_debug).

For a CLI request, run from your MediaWiki checkout:

```sh
mw dev mediawiki exec -- XDEBUG_SESSION=1 php maintenance/run.php Version
```

For an IDE that needs an explicit server mapping, add `PHP_IDE_CONFIG='serverName=YOUR_SERVER'` before `php` in that command.

## Inspect or disable Xdebug

```sh
mw dev mediawiki exec -- php --ri xdebug
mw dev env set MEDIAWIKI_XDEBUG_MODE off
mw dev mediawiki create --force-recreate
```

Restore step debugging with:

```sh
mw dev env set MEDIAWIKI_XDEBUG_MODE debug
mw dev mediawiki create --force-recreate
```

These environment changes also apply to the jobrunner. If it is running, recreate it with `mw dev mediawiki jobrunner create --force-recreate`.

`MEDIAWIKI_XDEBUG_MODE` and `MEDIAWIKI_XDEBUG_CONFIG` are the CLI settings; Compose passes them to PHP as `XDEBUG_MODE` and `XDEBUG_CONFIG`. Inspect the effective modes and host settings with `php --ri xdebug`. To enable profiling as well, select `debug,profile` and configure an output location and triggers as described in [Xdebug's profiling guide](https://xdebug.org/docs/profiler).
