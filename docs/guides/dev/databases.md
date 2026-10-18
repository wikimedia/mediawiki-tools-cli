<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/Databases?oldid=5181886 (CC BY-SA 4.0); see the page history for contributors. -->

# Use databases

The development environment comes with the ability to create wikis using sqlite, mysql or postgres.

If you follow the [first setup guide](setup.md) you will create a default wiki using a mysql service.

## Database web interfaces

PHPMyAdmin & Adminer are web interfaces for various database backends.

You can easily run these as part of the development environment.

```
mw dev phpmyadmin create
mw dev adminer create
```

## MySQL Replication

You can choose to add a mysql replica at any time. Replication will be handled automatically (but may take some time to catch up if you have already performed lots of actions).

```
mw dev mysql-replica create
```

## Import and export SQL

See [MySQL dumps](mysql-dumps.md) for import and export commands, including terminal handling and restoring MediaWiki data.

## Expose MySQL on the host

Use the bundled client or the [dump commands](mysql-dumps.md) without exposing a port. To connect a host database tool on port 3306:

```sh
mw dev env set MYSQL_PORT_3306 3306
mw dev mysql create --force-recreate
```

Choose an unused host port. The HTTP gateway's [PORT setting](ports.md) is separate.
