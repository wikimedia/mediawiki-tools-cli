# Jaeger service

Using this will automatically configure a jaeger service for MediaWiki.

This relies on code in MediaWiki that is a work in progress.
See https://phabricator.wikimedia.org/T340552 and https://gerrit.wikimedia.org/r/c/1027519 for more information.

When running, this will define:

```php
$wgOpenTelemetryConfig = [
    'samplingProbability' => 100, # a percentage despite the name
    'serviceName' => 'mediawiki',
    'endpoint' => 'http://jaeger:4318/v1/traces',
];
```
