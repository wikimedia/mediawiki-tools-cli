# Browse local logs

`mw dev logs` collects container stdout/stderr and MediaWiki `.log` files for the
current development context. Browse the copies in OpenSearch Dashboards; normal
Docker logging and original log files stay unchanged, even while collection runs.

## Start browsing

```sh
mw dev logs create
```

Open [local log browsing](http://localhost:8080/proxy/logs/), or follow the link on
your development dashboard if you use another gateway port. After logs arrive:

1. In Dashboards Management → Index patterns, create `mwcli-logs-*`.
2. Choose `@timestamp` as the time field.
3. Open Discover. Search `message` or filter by `service`, `stream`, or file `path`.

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
mw dev compose logs --tail=100 logs-opensearch logs-collector opensearch-dashboards
```
