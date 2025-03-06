# Postgres Service

## Exposing locally

To expose the Postgres service locally, you can set the `POSTGRES_PORT_5432` environment variable to a port on your host machine.

For example, to expose the Postgres service on port 5432:

```bash
mw docker env set POSTGRES_PORT_5432 5432
mw docker postgres create
```

