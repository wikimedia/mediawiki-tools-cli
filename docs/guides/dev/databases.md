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
