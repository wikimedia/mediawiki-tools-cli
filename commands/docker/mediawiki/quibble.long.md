# MediaWiki Quibble

Runs commands in a `quibble` container.

    This integration is WORK IN PROGRESS

    Gotchas:
        - This is a WORK IN PROGRESS integration, so don't expect all quibble features to work.
        - quibble will run tests for ALL checked out extensions by default.
        - If you let quibble touch your setup (missing --skip-install for example) it might break your environment.
        - quibble has various things hardcoded :(, for example the user and password for browser tests, you might find the below command helpful.

    mw docker mediawiki exec php maintenance/createAndPromote.php -- --sysop WikiAdmin testwikijenkinspass

## Documentation

 - [Quibble](https://doc.wikimedia.org/quibble/)