# Development environment

Develop MediaWiki and supporting Wikimedia services using Docker Compose.

Use `mw dev` to manage the environment. `mw docker` remains an alias for existing scripts.

You can start a basic MediaWiki MySQL development environment with the following commands:

```sh
mw dev mediawiki create
mw dev mysql create
mw dev mediawiki install --dbtype=mysql
```

You can:
 - Supplement this environment with additional services over time (See "Service commands" such as `redis`)
 - Recreate the entire environment in seconds (See `destroy`)
 - Run sites (See `mediawiki install`) and multiple environments simultaneously (See `--context`)
 - Run all common developer tools inside docker with ease (Eg. `mediawiki composer` / `fresh` / `quibble`)

## Suspending and resuming services

`mw dev stop` (alias `suspend`) saves the currently running services before
stopping them. `mw dev start` (alias `resume`) starts only that saved set that
still exists and is stopped. Services you had already stopped and completed
one-shot setup containers remain stopped. Repeating stop preserves the saved
set; repeating start with nothing left to resume is a no-op.

Explicit service create/start/stop/destroy commands update an existing record.
`mw dev restart` targets only currently running services. Global destroy
removes the record. Direct Docker/Compose commands bypass this intent tracking.

For compatibility, a context with no saved stop record initially starts all its
exited/created services. Its first global stop establishes the selective resume
set. State is kept separately for each development context.
