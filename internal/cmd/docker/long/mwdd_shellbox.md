# Shellbox

Shellbox is a library for command execution, and also a server and client for remote command execution.
Shellbox is usable starting with MediaWiki 1.36.

Different shellbox services include different libraries for different use cases.

The services provided by this command current make use of the Wikimedia Foundation
pre built containers https://docker-registry.wikimedia.org/wikimedia/mediawiki-libs-shellbox/tags/

For "automatic" configuration we follow a pattern similar to the Wikimedia production config.
If you run a service, you will find a service URL defined in $dockerServices.

```php
$dockerServices['shellbox-media'] = '<internal-url>';
```

This will also be connected to $wgShellboxUrls automatically if the service is running for any known
compatible URLs.

```php
$wgShellboxUrls['pagedtiffhandler'] = $dockerServices['shellbox-media'];
```

Please see the help text of individual shellbox services to see the exact details.

## Documentation

- [Shellbox](https://www.mediawiki.org/wiki/Shellbox)