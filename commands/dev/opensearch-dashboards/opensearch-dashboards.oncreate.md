# Local log browsing

The logging containers have been started; OpenSearch Dashboards may take another
minute to become ready. Open `/proxy/opensearch-dashboards/` on your development
gateway, or follow the service link on its dashboard.

Once MediaWiki has written logs, create the index pattern `mwcli-logs-*`, choose
`@timestamp` as the time field, and open Discover. Existing log files are unchanged.
