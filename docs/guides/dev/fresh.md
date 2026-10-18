# Run browser tests with Fresh

Fresh runs Node.js and browser tools in a container, with your MediaWiki checkout mounted at `/var/www/html/w`. Run the commands below from the checkout directory after [installing a wiki](setup.md).

## Install dependencies and run a test

```sh
mw dev mediawiki fresh npm ci
mw dev mediawiki fresh npm run selenium-test -- -- --spec tests/selenium/specs/recentchanges.js
```

Fresh sets `MW_SERVER=http://mediawiki-web:8080`, `MW_SCRIPT_PATH=/w`, `MEDIAWIKI_USER=Admin` and `MEDIAWIKI_PASSWORD=mwddpassword`. These are development defaults; override them for another wiki or account. The server address is internal to Docker and does not follow the host's HTTP port.

To inspect the container or override an environment variable for one command:

```sh
mw dev mediawiki fresh bash
mw dev mediawiki fresh MEDIAWIKI_USER=OtherAdmin npm run selenium-test -- -- --spec tests/selenium/specs/recentchanges.js
```

## Record a Selenium video

With a checkout that supports `wdio-mediawiki` video recording, run:

```sh
mw dev mediawiki fresh npm run selenium-test -- -- --recordVideo --useBrowserHeadless=false --maxInstances=1 --spec tests/selenium/specs/recentchanges.js
```

The bundled browser image provides Xvfb and ffmpeg. The runner starts its virtual display and recorder automatically; no manual Xvfb session is needed. Recording needs a non-headless browser inside that virtual display, so keep `--useBrowserHeadless=false`.

The first `--` passes the remaining flags through the CLI; the second forwards them through npm to WebdriverIO. Keep both when passing test-runner flags.

Videos and reports normally appear in `tests/selenium/log/` in your host checkout. Check for a newly written `.mp4` file after the run. A project's WebdriverIO configuration can change the output directory; `LOG_DIR` sets the default runner's log directory. Older checkouts may lack recording support or require their own setup; consult their Selenium README.

Recording is opt-in. Omit the recording options for normal browser tests. See [Fresh](https://github.com/wikimedia/fresh) and your checkout's `tests/selenium/README.md` for more options.
