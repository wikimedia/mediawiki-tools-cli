# MediaWiki Fresh

Run Node.js and browser tools in a Fresh container with your MediaWiki checkout
mounted at `/var/www/html/w`. Run commands from the checkout directory to preserve
the matching working directory inside the container.

Fresh sets `MW_SERVER=http://mediawiki-web:8080`, `MW_SCRIPT_PATH=/w`,
`MEDIAWIKI_USER=Admin`, and `MEDIAWIKI_PASSWORD=mwddpassword`. These are development
defaults; override them for another wiki or account by passing `NAME=value` before
the command. The server address is internal to Docker and does not follow the
host's HTTP port.

When forwarding npm test-runner flags, use `-- --`: the first separator passes
flags through this CLI and the second forwards them through npm to WebdriverIO.

## Recording

`mw dev mediawiki fresh record [spec]` runs `npm run selenium-test` with
`--recordVideo`, `--useBrowserHeadless=false`, and `--maxInstances=1`, adding
`--spec` when a path is provided. Omit the spec to record the full suite.
It uses the same container, wiki settings, and working directory as other Fresh
commands. Run `mw dev mediawiki fresh npm ci` first to install dependencies.

The bundled browser image supplies Xvfb and ffmpeg; the runner starts its virtual
display and recorder automatically. Recording requires a non-headless browser
inside that display. Normal test runs do not enable recording automatically.

Videos and reports normally appear in `tests/selenium/log/` in the host checkout.
A project's WebdriverIO configuration can change the output directory; `LOG_DIR`
sets the default runner's log directory. Older checkouts may lack recording
support or require their own setup; consult their Selenium README.

## Learn more

For a workflow installing dependencies, running tests, and checking a recording,
see `mw help guides dev fresh`.
