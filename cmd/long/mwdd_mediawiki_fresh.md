# MediaWiki Fresh

A [Fresh](https://github.com/wikimedia/fresh) environment is a fast and ready-to-use Docker container with various developer tools pre-installed.
Including Node.js, and headless browsers. It aims to help to run npm packages on your machine, without putting your personal data at risk!

Some default environment variables will be provided for you in the fresh cotnainer.

```sh
MW_SERVER=http://default.mediawiki.mwdd:${PORT}
MW_SCRIPT_PATH=/w
MEDIAWIKI_USER=Admin
MEDIAWIKI_PASSWORD=mwddpassword
```

## Usage

Note: the lack of `.localhost` at the end of the site name. Using `.localhost` will NOT work in this container.

Start an interactive terminal in the fresh container

```sh
fresh bash
```
  
Run npm ci in the currently directory (if within mediawiki)
```sh
fresh npm ci
```
  
Run mediawiki core tests (when in the mediawiki core directory)
```sh
fresh npm run selenium-test
```
  
Run a single Wikibase extension test spec (when in the Wikibase extension directory)
```sh
fresh npm run selenium-test:repo -- -- --spec repo/tests/selenium/specs/item.js
```

## Documentation

- [fresh](https://github.com/wikimedia/fresh)