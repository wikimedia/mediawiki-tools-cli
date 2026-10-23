# Browse local logs

`mw dev logs` collects container stdout/stderr and MediaWiki `.log` files for the
current development context. Browse the copies in OpenSearch Dashboards; normal
Docker logging and original log files stay unchanged, even while collection runs.

## Start browsing

```sh
mw dev logs create
```

Open [the MediaWiki log view](http://logs.local.wmftest.net:8080/proxy/logs/app/discover#/view/mwcli-mediawiki-logs),
or follow the link on your development dashboard if you use another gateway
port. Setup creates the `mwcli-logs-*` index pattern with `@timestamp` and a
saved MediaWiki view automatically. Existing saved views and preferences survive
subsequent starts.

Open your local wiki and reload a page or call its API. Refresh Discover and
check that the time range covers your request. You should see file events with
`source=mediawiki_file` and HTTP request/container events with `source=container`.
Search `message` or filter by `service`, `stream`, or file `path`. Clear the saved
service filter to browse other containers in this context.

## Use the Observability Logs explorer

`/proxy/logs/app/observability-logs#/` lists saved queries and visualizations;
"No Queries or Visualizations" does not mean no logs were collected. Click
**Event Explorer** to open the query interface. It does not open the saved
Discover view or automatically choose the log index. Enter:

```text
source = `mwcli-logs-*` | where service = 'mediawiki'
```

Choose a time range covering your request, then run the query. Remove the
`where` clause for all services, or use `service = 'mediawiki-web'` for HTTP
access logs. See [OpenSearch event analytics](https://docs.opensearch.org/2.19/observing-your-data/event-analytics/)
for PPL examples.

A running but idle wiki may not produce new entries. This is MediaWiki and
container logging; it does not capture the mwcli binary's host-terminal output.

## Stop or remove

```sh
mw dev logs stop    # Keep collected logs and file checkpoints
mw dev logs start
mw dev logs destroy # Delete collected data; preserve original logs
```

Container collection is live; output produced while the collector is stopped may
be missing. File collection catches up from retained files. There is no automatic
index retention, so delete old indexes in Dashboards when needed.

## Troubleshooting

Allow several GiB of spare Docker memory. Linux hosts may need
`vm.max_map_count=262144`; see [OpenSearch setup](https://docs.opensearch.org/latest/install-and-configure/install-opensearch/docker/).
Inspect startup or collection errors with:

```sh
mw dev compose logs -- --tail=100 logs-setup logs-opensearch logs-collector opensearch-dashboards
```

If startup failed, check `logs-setup` first. After fixing the cause, rerun
`mw dev logs create`. It fills in missing setup without overwriting saved-view
edits or user preferences. To reopen the supplied view after choosing another
homepage, use `/proxy/logs/app/discover#/view/mwcli-mediawiki-logs`.
