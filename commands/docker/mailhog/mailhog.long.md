# Mailhog service

[MailHog](https://github.com/mailhog/MailHog) is an email testing tool for developers.

Creating this service will automatically configure `$wgSMTP` for MediaWiki

```php
$wgSMTP = [
    'host'     => 'mailhog',
    'IDHost'   => 'mailhog',
    'port'     => '8025',
    'auth'     => false,
];
```

## Documentation

- [$wgSMTP](https://www.mediawiki.org/wiki/Manual:$wgSMTP)