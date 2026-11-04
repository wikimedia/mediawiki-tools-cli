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
Saved `LocalForward`, `RemoteForward` and `DynamicForward` entries matching the
Toolforge host are rejected: remove those entries for this host before starting
the tunnel. A saved `ClearAllForwardings yes` is overridden so the requested
replica forwards remain active.

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
The tunnel does not read, copy, display or save database credentials. The explicit
username helper below can retrieve just your replica username. `--tool` does not
apply to the tunnel: select the appropriate credentials in your SQL client.

Close database connections when you stop using them. The
[connection handling policy](https://wikitech.wikimedia.org/wiki/Help:Wiki_Replicas#Connection_handling_policy)
does not permit automatically maintained idle database connections or idle
connection pools. Keeping this SSH tunnel open alone does not open a database
connection; this command never logs into MariaDB or manages SQL connections.

## Retrieve your replica username

To print only the username from your Toolforge account's `~/replica.my.cnf`:

```sh
mw toolforge replicas username
mw toolforge replicas username --tool mytool
```

The helper extracts the `[client]` user on Toolforge using Python 3. Only the
username is returned over SSH; the password and credential file stay on Toolforge.
Nothing is saved locally. SSH diagnostics and errors go to stderr, and missing or
malformed files fail without printing their contents. The tunnel never retrieves
credentials automatically.

Use the username directly in a local SQL client with shell command substitution:

```sh
mariadb --host=127.0.0.1 --port=4711 --password enwiki_p \
  --user="$(mw --no-interaction toolforge replicas username)"
```

Keep the tunnel running in another terminal. For a tool account, add
`--tool mytool` to the username command inside the substitution.

## Experimental local authentication proxy

To let a local SQL client connect without receiving your real replica password:

```sh
mw toolforge replicas proxy enwiki
```

The proxy retrieves the replica username and password over your existing
Toolforge SSH access into the local CLI's memory. Neither is saved to disk, and
the real password is never printed. For a tool account, add `--tool mytool`.
The proxy runs on your computer and binds only to `127.0.0.1`, using port 4711
by default. Use `--port` to change it, or `--service web` to choose the web endpoint.
It accepts one wiki and cannot forward to arbitrary hosts.

Startup prints the selected database and local login: username `wmuser`
and a randomly generated password valid only while this proxy runs. Enter those
values in your SQL client's normal TCP connection settings. For example:

```sh
mariadb --host=127.0.0.1 --port=4711 --user=wmuser --password enwiki_p
```

Enter the **temporary local password** at the prompt. The CLI authenticates to
the replica on your behalf. The local credentials grant access through this
proxy; treat them as temporary secrets. OpenSSH keys/agent and host verification
are reused. The initial credential lookup can authenticate interactively; later
SSH channels use batch mode, so keys must be available without further prompts.

### Choose the local proxy login and port

The default is **user `wmuser` with a new random password** each time the proxy
starts. To accept the same credentials as the selected Toolforge replica account:

```sh
mw toolforge replicas proxy enwiki --tool mytool --use-replica-credentials
```

`--user` and `--password` override either local value independently. Explicit
values take precedence over `--use-replica-credentials`:

```sh
# Custom username; password stays random.
mw toolforge replicas proxy enwiki --user mylocaluser

# Keep the replica password, with a different local username.
mw toolforge replicas proxy enwiki --tool mytool --use-replica-credentials --user wmuser

# Override both local values.
mw toolforge replicas proxy enwiki --user mylocaluser --password 'example-password'
```

These options affect only the login accepted by the local proxy. Upstream
connections always authenticate with the selected account's replica credentials.
Only randomly generated passwords are printed. Supplied or reused passwords are
not printed at startup or saved to disk. `--password` values
are visible in process arguments; using `--use-replica-credentials` avoids putting
the real replica password into an argument.

Both commands use **`--port`, default 4711**:

```sh
# One proxy listener on 127.0.0.1:13306.
mw toolforge replicas proxy enwiki --port 13306

# Consecutive tunnel ports: enwiki on 13306, dewiki on 13307.
mw toolforge replicas tunnel enwiki dewiki --port 13306
```

### Connection lifetime

No database connection opens just because the proxy starts, a client logs in,
or a client sends a protocol ping. The first database operation opens a dedicated
SSH channel and replica connection for that client. At most four client sessions
are accepted, with no shared connection pool, automatic retries or keepalives.

Idle sessions close after **one minute**. `--idle-timeout 30s` changes this, within
1 second to 5 minutes. Protocol pings do not extend the deadline. Active queries
can finish even if they take longer than the idle timeout; the next idle interval
starts after the response. Disconnecting the client closes its upstream connection,
including while a query is running. Ctrl+C closes the listener and all sessions.
Idle expiry closes both sides, including an idle transaction or prepared statements;
reconnect explicitly to start a fresh session. The proxy never restores session
state or automatically reconnects an idle client. Configure your client to avoid
repeated background queries and automatic reconnects.

The prototype supports ordinary queries and prepared statements, including
executions that reuse cached parameter types and streamed string/blob parameters
(up to 16 MiB per statement). The frontend character set and
collation are preserved on the upstream connection.

`KILL` and client interrupt requests are rejected because local and upstream
connection IDs differ. Disconnect the original client to cancel its upstream
session. Leading executable SQL comments (`/*! ... */` and `/*M! ... */`) are
rejected too. Use `tunnel` with a normal SQL client when you need transparent
server protocol behavior.

Responses are buffered with a **16 MiB upstream data limit per command**; exceeding it closes
the session. Use a direct tunnel for large exports. Multiple statements/results,
LOCAL INFILE, compression, client TLS and cross-server joins are not supported.
The local hop uses loopback; the remote hop uses SSH. Database permissions remain
those of your replica account. This is a separate opt-in command; ordinary tunnels
and the username helper do not retrieve passwords.

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
