# Browse local logs with OpenSearch Dashboards

The optional `opensearch-dashboards` service supplies a local log browsing workflow
for [T292902](https://phabricator.wikimedia.org/T292902). It uses three containers:
Logstash reads existing MediaWiki log files, a dedicated OpenSearch instance indexes
them, and OpenSearch Dashboards provides Discover and visualizations.

This is a small development workflow inspired by
[Wikimedia logging](https://wikitech.wikimedia.org/wiki/Logstash), not a replica of
the production Kafka/rsyslog pipeline. The existing `opensearch` service remains
separate and continues to serve CirrusSearch.

## Start browsing

Use an existing context with MediaWiki installed:

```sh
mw dev opensearch-dashboards create
```

Open `/proxy/opensearch-dashboards/` on the development gateway (normally
`http://localhost:8080/proxy/opensearch-dashboards/`). The main dashboard also lists
the service. Wait for Dashboards to finish starting, make a request to your wiki,
then create an index pattern:

1. In Dashboards Management, open Index patterns and create `mwcli-logs-*`.
2. Select `@timestamp` as the time field.
3. Open Discover and select the new pattern. Search `message`, or filter by `path`.

The pattern becomes available after the first complete log line is indexed.
`@timestamp` records ingestion time, including when old files are imported. This
initial workflow preserves each line as `message`; multiline traces appear as
separate records. Valid JSON lines are also retained under `event` in the document
source, without indexing their arbitrary keys.

## Keep using the existing logs

No MediaWiki settings or Docker logging drivers change. The collector mounts the
same `${MEDIAWIKI_VOLUMES_LOGS:-mediawiki-logs}` source as MediaWiki, read-only, and
tails `**/*.log`. It also supports a host-directory override of that variable.
Existing file logs, `mw dev compose logs mediawiki`, and Docker stdout/stderr remain
available independently of the collector. Files outside that volume and container
stdout/stderr are not indexed. No Docker socket is mounted into the logging stack.

```sh
mw dev opensearch-dashboards stop    # Keep indexes and collector checkpoints
mw dev opensearch-dashboards start   # Catch up on files that still exist
mw dev opensearch-dashboards destroy # Delete only the stack's private data
```

MediaWiki continues writing its usual files during each action. Creating MediaWiki
alone does not start the logging stack. On first creation, retained files are read
from the beginning. Checkpoints and a bounded persistent delivery queue survive
collector restarts. Delivery can duplicate records after failures; logs rotated or
removed while the collector is stopped cannot be recovered. Destroying the logging
stack removes its checkpoints and indexes, so recreating it imports retained files
again. Global environment destruction still removes all environment data.

## Resources and access

The default JVM heaps are 512 MiB for OpenSearch and 256 MiB for Logstash, with
additional memory needed for Dashboards, native allocations and disk cache. Allow
several GiB of spare Docker memory. Linux Docker hosts may need
`vm.max_map_count=262144`; see the
[OpenSearch Docker setup](https://docs.opensearch.org/latest/install-and-configure/install-opensearch/docker/).
The pinned images offer both amd64 and arm64 Linux builds.

Search indexes grow until deleted: this first workflow does not configure automatic
retention. Avoid keeping it running indefinitely without monitoring disk use. Raw
logs can contain SQL, request data and credentials, just as the existing debug file
can. OpenSearch and the collector publish no host ports. The UI uses the existing
gateway and its tunnel authentication; the internal search security plugin is
disabled for local development.

Image overrides are `LOGS_OPENSEARCH_IMAGE`, `LOGS_COLLECTOR_IMAGE`, and
`OPENSEARCH_DASHBOARDS_IMAGE`. Keep OpenSearch and Dashboards versions compatible.
Heap overrides are `LOGS_OPENSEARCH_JAVA_OPTS` and `LOGS_COLLECTOR_JAVA_OPTS`.
Recreate the stack after changing them. The generic `image` command controls the
Dashboards image; use these environment overrides for the other two containers.
The gateway path is fixed at `/proxy/opensearch-dashboards/`.

## Troubleshooting

Inspect the three containers with the existing Compose wrapper:

```sh
mw dev compose logs --tail=100 logs-opensearch logs-collector opensearch-dashboards
```

If Dashboards is not ready, check OpenSearch health, available memory and the host
map-count setting. If no index appears, verify that MediaWiki is writing `.log`
files in the shared volume and that UID 1000 (the collector user) can read them.
The collector does not change permissions. Logs redirected by LocalSettings.php
outside this volume require a custom collector mount/configuration. Recreate the
collector if you change the MediaWiki log-volume source.

Implementation references:
[OpenSearch Logstash output](https://docs.opensearch.org/latest/tools/logstash/index/),
[Logstash file input](https://www.elastic.co/docs/reference/logstash/plugins/plugins-inputs-file).
