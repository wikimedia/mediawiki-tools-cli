# Develop a MediaWiki extension

Use `mw dev mediawiki get-code` to add an extension to the checkout used by your selected development context:

```sh
mw dev mediawiki get-code --extension Cite
```

The default Gerrit clone method uses SSH. Configure your Gerrit username first, or use `--gerrit-interaction-type http --no-commit-hook` for an anonymous public clone when you do not need to push changes.

Fetching the source does not enable the extension. Add it to `LocalSettings.php` after the shared development settings:

```php
wfLoadExtension( 'Cite' );
```

Run the updater when an extension needs database changes, then run a focused PHPUnit test from its directory:

```sh
mw dev mediawiki mwscript update
mw dev mediawiki composer phpunit extensions/Cite/tests/phpunit/unit/CiteParserTagHooksTest.php
```

If the test fails, inspect the MediaWiki service logs and repeat the test after the fix:

```sh
mw dev compose logs -- --tail=40 mediawiki
mw dev mediawiki composer phpunit extensions/Cite/tests/phpunit/unit/CiteParserTagHooksTest.php
```

The example uses Cite, but the same loop applies to other extensions and skins. Check the extension's own README for required Composer dependencies, configuration and test commands. See [MediaWiki configuration](configuration.md) and [service management](services.md) for more details.
