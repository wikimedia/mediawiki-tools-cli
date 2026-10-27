# Import and export MySQL dumps

The bundled `mysql` service runs MariaDB and supplies `mariadb` and `mariadb-dump`. These examples use its development
credentials, `root` / `toor`. A custom MySQL image may instead supply `mysql` and `mysqldump`.

Start the service and use an existing database, such as an installed wiki named `default`:

```sh
mw dev mysql create
```

No host database port needs to be exposed. Add `--context NAME` after `dev` when using another [context](contexts.md).

## Back up before destroying or changing the image

Export a dump before any destructive database operation — the data is not
otherwise preserved:

- `mw dev mysql destroy` removes the service's data volume outright.
- Changing `MYSQL_IMAGE` across a MariaDB major version (for example
  `10.11` to `11.4`) upgrades the on-disk data format one way, and cannot be
  downgraded afterward.

Both the primary and replica enable `MARIADB_AUTO_UPGRADE=1` so the image
runs any required system-table upgrade on startup. The image backs up the
system database before upgrading; this is not a backup of your wiki databases.

Take a dump with [Export a dump](#export-a-dump) first, recreate the service on
the new image, then reload it with [Import a dump](#import-a-dump). If you do
not need the existing data, skip the dump and reinstall with
`mw dev mediawiki install --dbtype=mysql`.

## Import a dump

Inspect the dump and back up existing data first: SQL dumps can replace tables or contain `DROP`, `CREATE DATABASE` and
`USE` statements. For a single-database dump without its own database-selection statements:

```sh
mw --no-interaction dev compose exec -- -T mysql mariadb -uroot -ptoor default < ./default.sql
```

The target database must already exist. `-T` disables the container terminal so SQL reaches stdin unchanged;
`--no-interaction` prevents CLI setup questions consuming that input. Omit the final `default` argument if the dump
selects its own databases. Changing that argument does not rename databases embedded in the SQL.

A MediaWiki restore does not include uploaded files or `LocalSettings.php`. Configure the intended wiki before
restoring, restore its uploads separately, then run its schema updates if required by the MediaWiki version:

```sh
mw dev mediawiki mwscript MW_DB=default update
```

## Export a dump

Write the dump inside the container, then copy it to the host:

```sh
mw --no-interaction dev compose exec -- -T mysql sh -c 'mariadb-dump -uroot -ptoor --single-transaction default > /tmp/wm-default.sql'
mw --no-interaction dev compose cp mysql:/tmp/wm-default.sql ./default.sql
mw --no-interaction dev compose exec -- -T mysql rm /tmp/wm-default.sql
```

This avoids mixing CLI startup notices into SQL redirected from the CLI's stdout. Replace `default` with your database
name. The dump contains that database's tables without `CREATE DATABASE` or `USE`, so it can be imported into another
existing database. `--single-transaction` gives a consistent snapshot of transactional tables; stop application writes
if non-transactional tables also need a consistent backup.

See [database services](databases.md) for web interfaces and replication.
