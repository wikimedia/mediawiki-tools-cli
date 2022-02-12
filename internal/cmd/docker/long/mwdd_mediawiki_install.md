# MediaWiki Install

Installs a new MediaWiki site MediaWiki maintenance scripts:
 - [install.php](https://www.mediawiki.org/wiki/Manual:Install.php)
 - [update.php](https://www.mediawiki.org/wiki/Manual:Update.php)

The sequence of actions is as follows:
 1) Ensure we know where MediaWiki is installed
 2) Ensure a `LocalSettings.php` file exists with the shim needed by this development environment
 3) Ensure composer dependencies are up to date, or run `composer install` &  `composer update`
 4) Move `LocalSettings.php` to a temporary location, as MediaWiki can't install with it present
 5) Wait for any needed databases to be ready
 6) Run `install.php`
 7) Move `LocalSettings.php` back
 8) Run `update.php`

## Manual installation

You can also run `install.php` and `update.php` manually, if you wish.

## Documentation

 - [install.php](https://www.mediawiki.org/wiki/Manual:Install.php)
 - [update.php](https://www.mediawiki.org/wiki/Manual:Update.php)
 - [LocalSettings.php](https://www.mediawiki.org/wiki/Manual:LocalSettings.php)
 - [Composer](https://www.mediawiki.org/wiki/Composer)