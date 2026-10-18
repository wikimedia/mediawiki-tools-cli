# Browse development logs

Collect container stdout/stderr and existing MediaWiki log files into a dedicated
OpenSearch instance, with OpenSearch Dashboards for browsing.

```sh
mw dev logs create
mw dev logs stop
mw dev logs start
```

Open `/proxy/logs/` on your development gateway. Create the index pattern
`mwcli-logs-*` using `@timestamp`, then open Discover. Filter by `service`,
`container_name`, `stream`, `source` or `path`.

Collection is limited to the current mwcli context and excludes the logging stack
itself. Existing Docker logging drivers, stdout/stderr, `docker logs` and MediaWiki
file output remain unchanged at all times. The collector reads copies via the
existing Docker API proxy and a read-only log-volume mount.

`destroy` deletes the search indexes and collector state, preserving original logs.
File checkpoints survive restarts; Docker collection is live and does not promise
backfill while stopped. See the local log browsing guide for details.

`opensearch-dashboards` remains an alias for `logs`.
