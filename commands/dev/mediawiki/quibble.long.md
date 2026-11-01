# MediaWiki Quibble

Runs commands in a `quibble` container. This integration is experimental:
Quibble runs tests for all checked-out extensions by default, and omitting
`--skip-install` can change your development installation.

For Selenium recording, run `quibble npm ci` first, then:

    mw dev mediawiki quibble record tests/selenium/specs/recentchanges.js

Omit the spec to record the full suite. The shortcut uses the internal development
wiki and `Admin`/`mwddpassword`, with videos in `tests/selenium/log/`. It skips
cloning, dependency installation and wiki installation. Run `quibble record --help`
for target, credential and output overrides.

The bundled container temporarily wraps Quibble to preserve environment overrides
for browser tests and custom commands ([T252883](https://phabricator.wikimedia.org/T252883)).
Raw Quibble commands otherwise retain their upstream defaults.

See [Quibble documentation](https://doc.wikimedia.org/quibble/) for its stages and options.
