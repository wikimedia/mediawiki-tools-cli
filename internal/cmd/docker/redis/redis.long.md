# Redis service

Using this service will automagically configure (but not use) an object cache in MediaWiki

```php
$wgObjectCaches['redis'] = [
	'class' => 'RedisBagOStuff',
	'servers' => [ 'redis:6379' ],
];
```

## Exposing locally

To expose the Redis service locally, you can set the `REDIS_PORT_6379` environment variable to a port on your host machine.

For example, to expose the Redis service on port 6379:

```bash
mw docker env set REDIS_PORT_6379 6379
mw docker redis create
```

## Documentation

- [Redis](https://www.mediawiki.org/wiki/Redis)
- [$wgObjectCaches](https://www.mediawiki.org/wiki/Manual:$wgObjectCaches)