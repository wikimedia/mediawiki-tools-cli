# Eventlogging service

You probably want to have the following extensions enabled for eventlogging to function.

```php
wfLoadExtensions( [
	'EventBus',
	'EventStreamConfig',
	'EventLogging',
	'WikimediaEvents'
  ] );
```

Using this will automatically configure a eventlogging server for MediaWiki.

```php
$wgEventServices = [ '*' => [ 'url' => 'http://eventlogging:8192/v1/events' ] ];
$wgEventServiceDefault = '*';
$wgEventLoggingStreamNames = false;
$wgEventLoggingServiceUri = "http://eventlogging.mwdd.localhost:" . parse_url($wgServer)['port'] . "/v1/events";
$wgEventLoggingQueueLingerSeconds = 1;
$wgEnableEventBus = defined( "MW_PHPUNIT_TEST" ) ? "TYPE_NONE" : "TYPE_ALL";
```

## Viewing logs

Checkout the logs of this service in order to see events coming in.

## Documentation

- [EventBus extension](https://www.mediawiki.org/wiki/Extension:EventBus)
- [EventStreamConfig extension](https://www.mediawiki.org/wiki/Extension:EventStreamConfig)
- [EventLogging extension](https://www.mediawiki.org/wiki/Extension:EventLogging)
- [WikimediaEvents extension](https://www.mediawiki.org/wiki/Extension:WikimediaEvents)