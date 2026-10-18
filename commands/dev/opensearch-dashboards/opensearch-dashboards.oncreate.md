# Local log browsing

The logging containers have been started; OpenSearch Dashboards may take another
minute to become ready. Open `/proxy/logs/` on your development gateway, or follow
the service link on its dashboard.

Once logs arrive, create the index pattern `mwcli-logs-*`, choose `@timestamp`,
and open Discover. Filter by `service` or `stream` to browse container output.
Normal Docker and MediaWiki logging remain unchanged while collection is enabled.
