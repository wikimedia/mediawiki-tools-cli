# MediaWiki Fresh

Run Node.js and browser tools in a Fresh container with your MediaWiki checkout mounted. Run commands from the checkout directory to preserve the matching working directory inside the container.

Fresh supplies development wiki connection settings. Use `-- --` before npm test-runner flags so they reach WebdriverIO.

See [Fresh browser tests and video recording](https://www.mediawiki.org/wiki/Cli/guide/Fresh) for dependencies, recording options and output files.
