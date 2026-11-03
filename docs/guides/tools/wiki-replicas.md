# Connect to Wiki Replicas from your computer

Use your existing Toolforge SSH access to reach one or more Wiki Replica
databases from a local SQL client. Run this command on your own computer, with
OpenSSH installed and your Toolforge login configured in your SSH config:

```sh
mw toolforge replicas tunnel enwiki dewiki commonswiki
```

The command requests these forwards through `login.toolforge.org`:

| Database | Local address | Wiki Replica destination |
| --- | --- | --- |
| `enwiki_p` | `127.0.0.1:4711` | `enwiki.analytics.db.svc.wikimedia.cloud:3306` |
| `dewiki_p` | `127.0.0.1:4712` | `dewiki.analytics.db.svc.wikimedia.cloud:3306` |
| `commonswiki_p` | `127.0.0.1:4713` | `commonswiki.analytics.db.svc.wikimedia.cloud:3306` |

SSH may ask you to verify the bastion's host key or unlock your SSH key, just as
with `mw toolforge shell`. Keep the command running and press **Ctrl+C** to close
all its forwards. It starts one local foreground SSH process, with no remote
shell, bastion daemon or automatic reconnection.

The printed mappings are requests: wait for SSH authentication before connecting
your SQL client. SSH checks that the local ports can be opened; the database's
availability and your database permissions are checked when the client connects.
An occupied local port causes SSH to exit rather than silently leave a partial
tunnel. Choose another starting port and retry:

```sh
mw toolforge replicas tunnel enwiki dewiki --port 13306
```

This maps `enwiki_p` to port 13306 and `dewiki_p` to 13307. Ports are consecutive
in argument order; the command does not probe for or silently choose free ports.
Names with the replica suffix, such as `enwiki_p`, are also accepted.

## Connect your SQL client

In another terminal, use the mapped port and database name:

```sh
mariadb --host=127.0.0.1 --port=4711 --user=YOUR_REPLICA_USER --password enwiki_p
```

Use **127.0.0.1**, not `localhost`: MariaDB clients may treat `localhost` as a Unix
socket instead of a TCP connection. A GUI client can use the same host, port and
database with a normal TCP connection; the CLI already provides the SSH tunnel.

Replica credentials are separate from your SSH login. Obtain them through your
existing Toolforge account or tool's `replica.my.cnf` workflow, as described in
[Wikitech's local connection guide](https://wikitech.wikimedia.org/wiki/Help:Wiki_Replicas#Connecting_to_the_database_replicas_from_your_own_computer).
This command does not read, copy, display or save database credentials. `--tool`
does not apply: select the appropriate replica credentials in your SQL client.

Close database connections when you stop using them. The
[connection handling policy](https://wikitech.wikimedia.org/wiki/Help:Wiki_Replicas#Connection_handling_policy)
does not permit automatically maintained idle database connections or idle
connection pools. Keeping this SSH tunnel open alone does not open a database
connection; this command never logs into MariaDB or manages SQL connections.

## Select the service or preview a command

The default `analytics` endpoint suits queries allowed to run longer. Use `web`
for the latency-sensitive queries described in
[Wikitech's naming conventions](https://wikitech.wikimedia.org/wiki/Help:Wiki_Replicas#Naming_conventions):

```sh
mw toolforge replicas tunnel enwiki --service web
mw toolforge replicas tunnel enwiki dewiki --port 13306 --dry-run
```

`--dry-run` prints mappings and the equivalent SSH command without opening a
connection. SSH configuration, keys, agents and host-key verification are reused;
the command creates no new authentication settings.

Destinations are restricted to per-wiki
`<wiki>.{analytics,web}.db.svc.wikimedia.cloud:3306` aliases. Full hostnames,
section names such as `s1`, extension databases and arbitrary forwarding options
are not accepted. All local listeners bind to `127.0.0.1`; the command does not
provide SOCKS, remote forwarding or publicly exposed database access. These are
production Wiki Replicas, so `--beta` and `--lima` do not apply.
