# Prometheus service

Using this will automatically configure a Prometheus service for MediaWiki.

When running, this will define:

```php
$wgStatsTarget = 'udp://statsd-exporter:8125';
$wgStatsFormat = 'dogstatsd';
```
