# OpenSearch Dashboards for local logs

Optionally browse existing MediaWiki log files using a dedicated OpenSearch,
Logstash and OpenSearch Dashboards stack. This is separate from the `opensearch`
service used by CirrusSearch.

```sh
mw dev opensearch-dashboards create
mw dev opensearch-dashboards stop
mw dev opensearch-dashboards start
```

Open `/proxy/opensearch-dashboards/` on your development gateway. After logs have
arrived, create the index pattern `mwcli-logs-*` with `@timestamp` as its time field,
then open Discover.

The collector reads `.log` files from MediaWiki's log volume without changing or
removing them. MediaWiki keeps writing its usual files whether this stack is
running, stopped or absent. Existing container stdout/stderr and CLI log commands
are unchanged. Container stdout/stderr is not collected by this stack.

`destroy` deletes the search indexes and collector checkpoints, preserving the
original MediaWiki logs. Recreating imports retained files again.

See the local log browsing guide for resources, limits and troubleshooting.
