# Explore pageviews

Get daily pageview counts for an article or an entire Wikimedia project. The command uses the public Wikimedia Analytics API and does not need a wiki login.

```sh
mw analytics pageviews 'Albert Einstein' --project en.wikipedia.org --agent user
```

Dates are inclusive and use `YYYYMMDD`. The default range is the last 30 days through yesterday. Pick a fixed range when comparing output formats:

```sh
mw analytics pageviews 'Albert Einstein' --project en.wikipedia.org --agent user \
  --start 20261001 --end 20261015 --output graph
```

Use the same query as structured data or CSV:

```sh
mw analytics pageviews 'Albert Einstein' --project en.wikipedia.org --agent user \
  --start 20261001 --end 20261015 --output json

mw analytics pageviews 'Albert Einstein' --project en.wikipedia.org --agent user \
  --start 20261001 --end 20261015 --output csv
```

Tables are shown in an interactive terminal; pipes receive JSON by default. Supported explicit formats are `table`, `json`, `jq`, `csv` and `graph`. TSV output is not currently available. Omit the article name to query project totals instead.
