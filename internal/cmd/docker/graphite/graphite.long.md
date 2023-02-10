# Graphite service

Creating this service will automatically configure `$wgStatsdServer` for MediaWiki.

```php
$wgStatsdServer = "graphite";
```

NOTE: The Graphite config differs between mwcli and graphite.wikimedia.org, see https://phabricator.wikimedia.org/T307366

## Documentation

- [$wgStatsdServer](https://www.mediawiki.org/wiki/Manual:$wgStatsdServer)