# php-rpc shellbox

Shellbox is a library for command execution, and also a server and client for remote command execution.
Shellbox is usable starting with MediaWiki 1.36.

For "automatic" configuration we follow a pattern similar to the Wikimedia production config.
If you run a service, you will find a service URL defined in $dockerServices.

```php
$dockerServices['shellbox-php-rpc'] = '<internal-url>';
```

Known ShellboxUrls are then also configured.
Currently that is the following Urls.

```php
$wgShellboxUrls['constraint-regex-checker'] = $dockerServices['shellbox-php-rpc'];
```

You can add more to your LocalSettings.php file if needed.

Note: This service will NOT be automatically loaded if MW_PHPUNIT_TEST is defined.
(This happens when you are running unit tests)

## Documentation

- [Shellbox](https://www.mediawiki.org/wiki/Shellbox)