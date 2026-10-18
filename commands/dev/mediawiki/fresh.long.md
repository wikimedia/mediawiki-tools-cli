# MediaWiki Fresh

Run Node.js and browser tools in a Fresh container with your MediaWiki checkout mounted. Run commands from the checkout directory to preserve the matching working directory inside the container.

Fresh supplies development wiki connection settings. Use `-- --` before npm test-runner flags so they reach WebdriverIO.

Use `mw dev mediawiki fresh record [spec]` to record Selenium tests. This runs `npm run selenium-test` with video recording enabled, a non-headless browser and one worker. Omit the spec to record the full suite. Run `mw dev mediawiki fresh npm ci` first.

See `mw help guides dev fresh` for dependencies, recording options and output files.
