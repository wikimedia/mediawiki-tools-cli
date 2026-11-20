# Read Grafana dashboards and query metrics

Use `mw grafana` to access public dashboards and data sources on Wikimedia Grafana without signing in. Dashboard JSON contains panel definitions and queries. To retrieve actual samples, query a data source.

## Find a dashboard and save its JSON

Search by title and copy the UID from the results.

```sh
mw grafana dashboard list 'MediaWiki' -o jq --format '.[] | {uid, title, url}'
mw grafana dashboard get VIfr7aank > dashboard.json
```

Replace the example UID with the dashboard you want. You can also copy it from the segment after `/d/` in a Grafana dashboard URL. Use `--page` and `--limit` to page through search results. `dashboard get` prints the dashboard model. Use `mw grafana api /api/dashboards/uid/VIfr7aank` to include Grafana's response metadata.

## Inspect and query an existing panel

List the panels on the MediaWiki LinksUpdate dashboard. Then use a panel title to see its saved query and retrieve the last 15 minutes of samples.

```sh
mw grafana dashboard panels 'MediaWiki LinksUpdate'
mw grafana dashboard panels 'MediaWiki LinksUpdate' --panel 'RefreshLinksJob good outcome'
mw grafana dashboard query 'MediaWiki LinksUpdate' 'RefreshLinksJob good outcome' --since 15m
```

Titles are case-insensitive. Use a full title or an unambiguous fragment. If several dashboards or panels match, choose an ID from the suggestions. `dashboard query` runs the panel's visible Prometheus queries and returns Grafana data frames. Use `--var name=value` for variables with no single saved value, such as **All** or a multi-select. Pass values as they should appear in the expression. For example, get the Varnish 2xx response rate at `eqiad` from the CDN Frontend Traffic dashboard.

```sh
mw grafana dashboard query 'CDN Frontend Traffic' 'Varnish status' --since 15m \
  --var site=eqiad --var cache_type=text --var status_type=2
```

If the panel uses another kind of data source or a Grafana-specific variable format, copy its request from **Inspect → Query** in the browser and send it with `mw grafana api /api/ds/query --input query.json`.

## Query Prometheus data

List the Prometheus data sources and choose the UID for the site and service you want.

```sh
mw grafana datasource list -o jq --format '.[] | select(.type == "prometheus") | {uid, name}'
mw grafana query 'vector(1)' --datasource 000000019
mw grafana query 'vector(1)' --datasource 000000019 --since 15m --step 1m
```

`vector(1)` returns a constant sample you can use to check connectivity. The first query returns the current value. The second returns samples over the last 15 minutes at one-minute intervals. Filter the Prometheus response with `-o jq --format '.data.result'`.

## Call the Grafana API directly

Pass a relative Grafana API endpoint. Use `-f` with `-X GET` to add query parameters, or `--input` to send a JSON request to Grafana's data source query endpoint.

```sh
mw grafana api /api/health
mw grafana api /api/search -X GET -f type=dash-db -f query='MediaWiki'
mw grafana api /api/ds/query -X POST --input query.json
```

For the third example, save this native Grafana request as `query.json`.

```json
{
  "from": "now-5m",
  "to": "now",
  "queries": [
    {
      "refId": "A",
      "datasource": {"type": "prometheus", "uid": "000000019"},
      "expr": "vector(1)",
      "instant": true,
      "range": false,
      "intervalMs": 60000,
      "maxDataPoints": 10
    }
  ]
}
```

Grafana data sources use different query fields. Use the request shown by the panel inspector to reproduce a specific panel query. [Grafana data source API reference](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/api-legacy/data_source/)
