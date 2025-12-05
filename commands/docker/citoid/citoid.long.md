# Citoid service

Using this will automatically configure a citoid service for MediaWiki.

When running, this will define:

```php
$wgCitoidServiceUrl = 'http://citoid.local.wmftest.net:8080/api';
```

A common usecase for this service requires installation of some additional extensions.

```sh
mwdev dev mediawiki get-code --extension Citoid --extension Cite --extension VisualEditor --extension TemplateData
```

You would need to load these in your local settings:

```php
wfLoadExtension( 'VisualEditor' );
wfLoadExtension( 'Cite' );
wfLoadExtension( 'Citoid' );
wfLoadExtension( 'TemplateData' );
```

And also import some default pages:

```sh
mw docker mediawiki mwscript importDump https://gitlab.wikimedia.org/repos/ci-tools/patchdemo/-/raw/bc3e798b6bbbc3354d8b957456b87a50c8150853/pages/extensions-Citoid.xml
mw docker mediawiki mwscript importDump https://gitlab.com/wmde/technical-wishes/docker-dev/-/raw/c40990e67e293fa2026a2c67c9963fe3a1b2608a/modules/Citoid/xml-dumps/Citoid-templates.xml
```