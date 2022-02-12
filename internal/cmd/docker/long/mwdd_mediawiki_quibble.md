# MediaWiki Quibble

Runs commands in a `quibble` container.
	
	This integration is WORK IN PROGRESS`,
	Example: `        # Start an interactive terminal in the quibble container
	quibble bash
  
	# Get help for the quibble CLI tool
	quibble quibble -- --help

	# Run php-unit quibble stage using your mwdd LocalSettings.php, skipping anything that alters your installation
	quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --run phpunit-unit

	# Run composer phpunit:unit inside the quibble container
	quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --command "composer phpunit:unit"

    Gotchas:
        - This is a WORK IN PROGRESS integration, so don't expect all quibble features to work.
        - quibble will run tests for ALL checked out extensions by default.
        - If you let quibble touch your setup (missing --skip-install for example) it might break your environment.
		- quibble has various things hardcoded :(, for example the user and password for browser tests, you might find the below command helpful.

	mw docker mediawiki exec php maintenance/CeateAndPromote.php -- --sysop WikiAdmin testwikijenkinspass

## Documentation

 - [Quibble](https://doc.wikimedia.org/quibble/)