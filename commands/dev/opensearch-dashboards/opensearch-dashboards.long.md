# Browse development logs

Collect container stdout/stderr and existing MediaWiki log files into a dedicated
OpenSearch instance, with OpenSearch Dashboards for browsing.

```sh
mw dev logs create
mw dev logs stop
mw dev logs start
```

Open `/proxy/logs/` on your development gateway. Setup creates the
`mwcli-logs-*` index pattern using `@timestamp` and a saved "MediaWiki logs"
view in Discover. The first visit opens that view, showing MediaWiki file logs,
PHP/container output, web requests and jobrunner output. Browse other services
by clearing its service filter. Filter by `service`, `container_name`, `stream`,
`source` or `path`. Existing saved views and user settings are preserved.

Observability → Logs lists saved queries, not incoming log entries. Click
Event Explorer, run this PPL query, and select a time range covering your wiki
requests:

```text
source = `mwcli-logs-*` | where service = 'mediawiki'
```

Generate a request by opening your wiki or calling its API. This stack collects
MediaWiki and its development containers, not the CLI process running in your
host terminal. A running but idle wiki may produce no new entries.

Collection is limited to the current mwcli context and excludes the logging stack
itself. Existing Docker logging drivers, stdout/stderr, `docker logs` and MediaWiki
file output remain unchanged at all times. The collector reads copies via the
existing Docker API proxy and a read-only log-volume mount.

`destroy` deletes the search indexes and collector state, preserving original logs.
The one-shot `logs-setup` container prepares the index template and dashboard
objects before collection starts; inspect its output if initialization fails.
File checkpoints survive restarts; Docker collection is live and does not promise
backfill while stopped. See the local log browsing guide for details.

`opensearch-dashboards` remains an alias for `logs`.
