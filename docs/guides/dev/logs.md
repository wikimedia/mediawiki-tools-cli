# Browse local logs

The optional `mw dev logs` workflow collects **container stdout/stderr and MediaWiki
file logs** for the current development context. A Vector collector sends copies
to a dedicated OpenSearch instance; OpenSearch Dashboards provides Discover and
visualizations. The existing CirrusSearch `opensearch` service stays separate.

Normal logging remains unchanged **whether collection is running, stopped or
absent**. Nothing replaces Docker's logging driver, redirects application output,
or changes MediaWiki settings. `docker logs`, `mw dev compose logs`, and the
original files continue working. The collector does not consume or truncate them.

This implements a local workflow for
[T292902](https://phabricator.wikimedia.org/T292902), inspired by
[Wikimedia logging](https://wikitech.wikimedia.org/wiki/Logstash), without replicating
the production Kafka/rsyslog pipeline. Vector supplies Docker API and file inputs;
Logstash is not required for this workflow.

## Start browsing

```sh
mw dev logs create
```

Open `/proxy/logs/` on your gateway, normally
`http://localhost:8080/proxy/logs/`. The main development dashboard also links to
it. Wait for Dashboards to start and generate some application output, then:

1. In Dashboards Management, open Index patterns and create `mwcli-logs-*`.
2. Select `@timestamp` as the time field.
3. Open Discover and choose the new pattern.

Search `message`, or filter by `service` (such as `mysql`, `mediawiki-web`, or a
custom Compose service), `container_name`, `stream` (`stdout`/`stderr`), `source`
(`container`/`mediawiki_file`), and file `path`. Original lines remain searchable;
this workflow does not parse application-specific JSON fields or combine multiline
stack traces. Docker timestamps are preserved; file timestamps represent collection
time. Docker may merge stdout/stderr for containers created with a TTY.

`mw dev opensearch-dashboards` remains an alias for `mw dev logs`. The actual
Dashboards Compose service is still named `opensearch-dashboards`.

## What is collected

The collector reads Docker's existing log API through the gateway's GET-only
`docker-socket-proxy`. It selects containers by their exact Compose project label,
including custom services in the context, and follows container lifecycle events.
It excludes `logs-collector`, `logs-opensearch`, and `opensearch-dashboards` to avoid
feedback loops. Containers from other contexts and unrelated projects are excluded.
No Docker socket is mounted into the collector; the existing proxy's allowed GET
endpoints have daemon-wide read access, while collection is scoped by labels.

The file input mounts `${MEDIAWIKI_VOLUMES_LOGS:-mediawiki-logs}` read-only, matching
MediaWiki's source, including a host-directory override. It reads `**/*.log`, keeping
checkpoints in the collector's own volume. This captures debug logs not written to
stdout/stderr. Files redirected outside that volume are not collected. Lines over
1 MiB are skipped by the file input. Container labels are not stored in the search
index; the collector retains project/service identity and standard container metadata.

Docker must support reading the selected container's logs. The normal `json-file`
and `local` drivers do; other drivers may need Docker's dual-logging cache. The
collector does not change a driver's settings if it cannot read its output.

## Stop, resume and remove

```sh
mw dev logs stop    # Keep indexes, file checkpoints and buffered events
mw dev logs start
mw dev logs destroy # Delete only the logging stack's private data
```

Applications continue logging during each action. Creating an application service
alone does not enable collection. File collection catches up from retained files
and checkpoints. **Docker collection is live:** this workflow does not promise
historical backfill across collector downtime, or complete capture of very
short-lived containers. Original logs remain accessible through Docker. Files
rotated or removed during downtime cannot be recovered by the file input either.

A bounded 512 MiB disk buffer retains collected events during temporary OpenSearch
outages. Collection can still lose or duplicate records on failures; this is a
local debugging aid. Destroy deletes indexes, buffered events and file checkpoints;
recreating imports retained files again. Global environment destruction continues
to remove all environment data.

## Resources and access

OpenSearch uses a 512 MiB JVM heap; allow additional memory for its native
allocations, Dashboards, Vector and disk cache. Allow several GiB of spare Docker
memory. Linux hosts may need `vm.max_map_count=262144`; see the
[OpenSearch Docker setup](https://docs.opensearch.org/latest/install-and-configure/install-opensearch/docker/).
The pinned images support Linux amd64 and arm64.

There is no automatic index retention yet; monitor disk usage or delete old indexes
in Dashboards. Raw logs may contain SQL, request data and credentials, as their
original outputs do. OpenSearch and the collector publish no host ports. The UI
uses the existing gateway and its tunnel authentication. Search's internal security
plugin is disabled for this local workflow.

Override images with `LOGS_OPENSEARCH_IMAGE`, `LOGS_COLLECTOR_IMAGE` (Vector), or
`OPENSEARCH_DASHBOARDS_IMAGE`; keep OpenSearch and Dashboards versions compatible.
The OpenSearch heap override is `LOGS_OPENSEARCH_JAVA_OPTS`. Recreate the stack to
apply changes. The `mw dev logs image` command controls the Dashboards image.

## Troubleshooting

```sh
mw dev compose logs --tail=100 logs-opensearch logs-collector opensearch-dashboards
```

For startup failures, check available memory and the host map-count setting. For
missing container output, check `mw dev compose logs SERVICE` and the collector's
Docker API diagnostics. Check that both containers carry the current Compose
project label, and that the Docker logging driver supports log reads.

For missing file logs, verify that MediaWiki is writing `.log` files in the shared
volume and that the collector can read them. It does not change file permissions.
Custom paths require a custom collector mount/configuration; recreate the collector
if you change the MediaWiki log-volume source.

Implementation references:
[Vector Docker input](https://vector.dev/docs/reference/configuration/sources/docker_logs/),
[file input](https://vector.dev/docs/reference/configuration/sources/file/), and
[OpenSearch-compatible output](https://vector.dev/docs/reference/configuration/sinks/elasticsearch/).
