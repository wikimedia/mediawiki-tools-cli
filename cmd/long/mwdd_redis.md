# Redis service

Using this service will automagically configure (but not use) an object cache in MediaWiki

```php
$wgObjectCaches['redis'] = [
	'class' => 'RedisBagOStuff',
	'servers' => [ 'redis:6379' ],
];
```

## Documentation

- [Redis](https://www.mediawiki.org/wiki/Redis)
- [$wgObjectCaches](https://www.mediawiki.org/wiki/Manual:$wgObjectCaches)