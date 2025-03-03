# Mysql Service

## Exposing locally

To expose the MySQL service locally, you can set the `MYSQL_PORT_3306` environment variable to a port on your host machine.

For example, to expose the MySQL service on port 3306:

```bash
mw docker env set MYSQL_PORT_3306 3306
mw docker mysql create
```
