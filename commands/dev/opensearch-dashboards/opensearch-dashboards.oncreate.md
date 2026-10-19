# Local log browsing

Open `/proxy/logs/app/discover#/view/mwcli-mediawiki-logs` on your development
gateway, or follow the service link on its dashboard. The index pattern and
saved MediaWiki log view are ready; existing views and settings are preserved.

Open your wiki or call its API to generate fresh logs. If the view is empty,
check its time range and refresh. To browse all collected services, clear the
service filter. Normal Docker and MediaWiki logging remain unchanged.

In Observability → Logs, click Event Explorer, select a time range and run:

```text
source = `mwcli-logs-*` | where service = 'mediawiki'
```
