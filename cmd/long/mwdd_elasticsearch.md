# Elasticsearch service

Using this will automatically configure an elasticsearch server for MediaWiki via the [CirrusSearch](https://www.mediawiki.org/wiki/Extension:CirrusSearch) extension.
In order for this to do anything you will need to CirrusSearch extension installed and enabled.

```php
$wgCirrusSearchServers = [ 'elasticsearch' ];
```

In order to configure a search index for a wiki, you'll need to run some maintenance scripts:

```sh
# Configure the search index and populate it with content
php extensions/CirrusSearch/maintenance/UpdateSearchIndexConfig.php
php extensions/CirrusSearch/maintenance/ForceSearchIndex.php --skipLinks --indexOnSkip
php extensions/CirrusSearch/maintenance/ForceSearchIndex.php --skipParse
```

And you'll need to process the job queue any time you add/update content and want it updated in ElasticSearch

```sh
php maintenance/runJobs.php
```

## Documentation

- [CirrusSearch extension](https://www.mediawiki.org/wiki/Extension:CirrusSearch)
- [runJobs.php](https://www.mediawiki.org/wiki/Manual:RunJobs.php)