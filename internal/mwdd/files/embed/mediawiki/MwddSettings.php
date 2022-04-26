<?php

// Set a umask for MediaWiki as we are in a development envrionment
// This is also currently injected via a wrapper around install.php for initial setup
umask(000);

################################
# MWDD START
################################

# Protect against web entry
if ( !defined( 'MEDIAWIKI' ) ) {
	exit;
}

################################
# MWDD Setup
################################

# When used via CLI, use the default DB if no MW_DB is specified
# Maintenance scripts with --wiki passed will set MW_DB
if ( PHP_SAPI === 'cli' && !defined( 'MW_DB' ) ) {
    define( 'MW_DB', 'default' );
}

# Detect usage of update.php, so we can turn of replication https://phabricator.wikimedia.org/T283417
$dockerIsRunningUpdate = false;
# Sometimes argv is not set, such as when running php built in web server via quibble
if(array_key_exists('argv', $_SERVER)){
	$dockerIsRunningUpdate = basename( $_SERVER['argv'][0] ) === 'update.php';
}

# Must be above WebRequest::detectServer.
# mwdd uses a proxy server with no default ports.
$wgAssumeProxiesUseDefaultProtocolPorts = false;

# Either use the MW_DB env var, or get the DB from the request
if ( defined( "MW_DB" ) ) {
    $dockerDb = MW_DB;
    $wgServer = "//$dockerDb.mediawiki.mwdd.localhost:80";
} elseif( array_key_exists( 'SERVER_NAME', $_SERVER ) ) {
    $dockerHostParts = explode( '.', $_SERVER['SERVER_NAME'] );
    $dockerDb = $dockerHostParts[0];
    $wgServer = WebRequest::detectServer();
} else {
    die( 'Unable to decide which MediaWiki DB to use (from env or request).' );
}

# Only use "advanced" services if they can be seen, and if we are not in tests
# TODO this is confusing, tidy it up, what is it?
# TODO do we want this running in phpunit tests? maybe we need a way to specify that we do?
# TODO cache these existance checks for at least 1 second to avoid hammering looking for these services..
$dockerServices = [
	'mysql' => gethostbyname('mysql') !== 'mysql',
	'mysql-replica' => gethostbyname('mysql-replica') !== 'mysql-replica' && !defined( 'MW_PHPUNIT_TEST' ) && !$dockerIsRunningUpdate,
	'eventlogging' => gethostbyname('eventlogging') !== 'eventlogging' && !defined( 'MW_PHPUNIT_TEST' ),
	'redis' => gethostbyname('redis') !== 'redis' && !defined( 'MW_PHPUNIT_TEST' ),
	'memcached' => gethostbyname('memcached') !== 'memcached' && !defined( 'MW_PHPUNIT_TEST' ),
	'elasticsearch' => gethostbyname('elasticsearch') !== 'elasticsearch' && !defined( 'MW_PHPUNIT_TEST' ),
	'graphite' => gethostbyname('graphite') !== 'graphite' && !defined( 'MW_PHPUNIT_TEST' ),
	'mailhog' => gethostbyname('mailhog') !== 'mailhog' && !defined( 'MW_PHPUNIT_TEST' ),
	'shellbox-media' => gethostbyname('shellbox-media') !== 'shellbox-media' && !defined( 'MW_PHPUNIT_TEST' ),
	'shellbox-php-rpc' => gethostbyname('shellbox-php-rpc') !== 'shellbox-php-rpc' && !defined( 'MW_PHPUNIT_TEST' ),
	'shellbox-score' => gethostbyname('shellbox-score') !== 'shellbox-score' && !defined( 'MW_PHPUNIT_TEST' ),
	'shellbox-syntaxhighlight' => gethostbyname('shellbox-syntaxhighlight') !== 'shellbox-syntaxhighlight' && !defined( 'MW_PHPUNIT_TEST' ),
	'shellbox-timeline' => gethostbyname('shellbox-timeline') !== 'shellbox-timeline' && !defined( 'MW_PHPUNIT_TEST' ),
];

################################
# MWDD Database
################################
// TODO cache the DB existance checks somehow so as not to run on every request...

// Figure out if we are using sqlite, or if this should be mysql..?
if( file_exists( $IP . '/cache/docker/' . $dockerDb . '.sqlite' ) ) {
	$dockerDbType = 'sqlite';
}

// Figure out if we are using mysql
if(!isset($dockerDbType)){
	try{
		$mysqlPdo = new PDO(
			"mysql:host=mysql;dbname=" . $dockerDb,
			'root',
			'toor',
			[
				PDO::ATTR_TIMEOUT => 1, // in seconds
			]
		);
		$mysqlCheck = $mysqlPdo->query("SHOW DATABASES LIKE \"" . $dockerDb . "\"");
		if($mysqlCheck === false) {
			var_dump(json_encode($mysqlPdo->errorInfo()));
			die("Failed to get mysql databases list looking for {$dockerDb}.");
		}
		if(count($mysqlCheck->fetchAll()) === 1){
			$dockerDbType = 'mysql';
		}
	} catch ( Exception $e ) {
		// TODO set the timeout on connection to be much shorter, so that when mysql doesnt exist, it doenst hang for a while
		// do nothing
	}
}

// Figure out if we are using postgres
if(!isset($dockerDbType)){
	$pvars = [
		'dbname' => $dockerDb,
		'user' => 'root',
		'password' => 'toor',
		'host' => 'postgres',
	];
	$pconnString = '';
	foreach ( $pvars as $name => $value ) {
		$pconnString .= "$name='" . str_replace( "'", "\\'", $value ) . "' ";
	}
	$postgresConn = @pg_connect( $pconnString . ' connect_timeout=1', PGSQL_CONNECT_FORCE_NEW );
	if($postgresConn !== false){
		$dockerDbType = 'postgres';
	}
}

// Otherwise something must be wrong
if(!isset($dockerDbType)){
	$message = "<html><head></head><body>" . PHP_EOL;
	$message .= "<h1>mwcli: Unable to find database</h1>" . PHP_EOL;
	$message .= '<p>Could not find a running database for the database name <pre>' . $dockerDb . '</pre></p>' . PHP_EOL;
	$message .= '<p>Please ensure that the site is installed and the database service chosen is running.</p>' . PHP_EOL;
	$message .= '<p>You can check running services with <pre>mw docker docker-compose ps</pre></p>' . PHP_EOL;
	$message .= '<p>You can install a new site with <pre>mw docker mediawiki install</pre></p>' . PHP_EOL;
	$message .= '</body></html>' . PHP_EOL;
	die($message);
}

$wgDBname = $dockerDb;

if( $dockerDbType === 'sqlite' ) {
	$wgDBservers = [
		[
			'dbDirectory' => $IP . '/cache/docker',
			'dbname' => $dockerDb,
			'type' => $dockerDbType,
			'flags' => DBO_DEFAULT,
			'load' => 1,
		],
	];
}

if( $dockerDbType === 'mysql' ) {
	$wgDBservers = [
		[
			'host' => "mysql",
			'dbname' => $dockerDb,
			'user' => 'root',
			'password' => 'toor',
			'type' => $dockerDbType,
			'flags' => DBO_DEFAULT,
			'load' => $dockerServices['mysql-replica'] ? 0 : 1,
		],
	];
	if($dockerServices['mysql-replica'] ) {
		$wgDBservers[] = [
			'host' => "mysql-replica",
			'dbname' => $dockerDb,
			'user' => 'root',
			'password' => 'toor',
			'type' => $dockerDbType,
			'flags' => DBO_DEFAULT,
			# Avoid switching to readonly too early (for example during update.php)
			'max lag' => 60,
			'load' => 1,
		];
	}

	// mysql only stuff (would need to change for sqlite?)
	$wgDBprefix = "";
	$wgDBTableOptions = "ENGINE=InnoDB, DEFAULT CHARSET=binary";
}

if( $dockerDbType === 'postgres' ) {
	$wgDBservers = [
		[
			'host' => "postgres",
			'dbname' => $dockerDb,
			'user' => 'root',
			'password' => 'toor',
			'type' => $dockerDbType,
			'flags' => DBO_DEFAULT,
			'load' => 1,
		],
	];
	// https://www.mediawiki.org/wiki/Manual:$wgDBmwschema
	$wgDBmwschema = "mediawiki";
}

################################
# MWDD Mail / Mail Hog
################################
$wgSMTP = [
    'host'     => 'mailhog',
    'IDHost'   => 'mailhog',
    'port'     => '1025',
    'auth'     => false,
];

################################
# MWDD Redis
################################
if(gethostbyname('redis') !== 'redis') {
	$wgObjectCaches['redis'] = [
		'class' => 'RedisBagOStuff',
		'servers' => [ 'redis:6379' ],
	];
}

################################
# MWDD Memcached
################################
if(gethostbyname('memcached') !== 'memcached') {
	$wgMemCachedServers = [ 'memcached:11211' ];
}

################################
# MWDD ElasticSearch
################################
if(gethostbyname('elasticsearch') !== 'elasticsearch') {
	$wgCirrusSearchServers = [ 'elasticsearch' ];
}

################################
# MWDD EventLogging
################################
if(gethostbyname('eventlogging') !== 'eventlogging') {
	$wgEventServices = [
		'*' => [ 'url' => 'http://eventlogging:8192/v1/events' ],
	];
	$wgEventServiceDefault = '*';
	$wgEventLoggingStreamNames = false;
	$wgEventLoggingServiceUri = "http://eventlogging.mwdd.localhost:" . parse_url($wgServer)['port'] . "/v1/events";
	$wgEventLoggingQueueLingerSeconds = 1;
	$wgEnableEventBus = defined( "MW_PHPUNIT_TEST" ) ? "TYPE_NONE" : "TYPE_ALL";
}

################################
# MWDD Graphite & Statsd
################################
if(gethostbyname('graphite-statsd') !== 'graphite-statsd') {
	$wgStatsdServer = "graphite-statsd";
}

################################
# Shellboxes
################################
if($dockerServices['shellbox-media']) {
	$wgShellboxUrls['pagedtiffhandler'] = $dockerServices['shellbox-media'];
	$wgShellboxUrls['pdfhandler'] = $dockerServices['shellbox-media'];
}
if($dockerServices['shellbox-php-rpc']) {
	$wgShellboxUrls['constraint-regex-checker'] = $dockerServices['shellbox-php-rpc'];
}
if($dockerServices['shellbox-score']) {
	$wgShellboxUrls['score'] = $dockerServices['shellbox-score'];
}
if($dockerServices['shellbox-syntaxhighlight']) {
	$wgShellboxUrls['syntaxhighlight'] = $dockerServices['shellbox-syntaxhighlight'];
}
if($dockerServices['shellbox-timeline']) {
	$wgShellboxUrls['easytimeline'] = $dockerServices['shellbox-timeline'];
}

################################
# MWDD Special Page
################################
require_once __DIR__ . '/MwddSpecialPage.php';
$wgSpecialPages['Mwdd'] = MwddSpecial::class;
$wgExtensionMessagesFiles['Mwdd'] = __DIR__ . '/MwddSpecialPage-aliases.php';

################################
# MWDD Dev Settings
################################
$wgShowHostnames = true;

## Site settings
$wgScriptPath = "/w";
$wgSitename = "mwdd-$dockerDb";
$wgMetaNamespace = "Project"; // Set to "Project", instead of the default $wgSitename
// TODO re add favicon (removed porting to go)
//$wgFavicon = "{$wgScriptPath}/.docker/favicon.ico";

## Various directories
$wgUploadDirectory = "{$IP}/images/docker/{$dockerDb}";
$wgCacheDirectory = "{$IP}/cache/docker/{$dockerDb}";
$wgTmpDirectory = "{$wgCacheDirectory}";

$wgUploadPath = "{$wgScriptPath}/images/docker/{$dockerDb}";

## Dev & Debug
$dockerLogDirectory = "/var/log/mediawiki";
$wgDebugLogFile = "$dockerLogDirectory/debug.log";

error_reporting( -1 );
ini_set( 'display_errors', 1 );
$wgShowExceptionDetails = true;
$wgShowSQLErrors = true;
$wgDebugDumpSql  = true;
$wgShowDBErrorBacktrace = true;
$wgDevelopmentWarnings = true;
$wgEnableJavaScriptTest = true;

## Email

# TODO use some mail catcher?
$wgEnableEmail = true;
$wgEmergencyContact = "mediawiki@$dockerDb";
$wgPasswordSender = "mediawiki@$dockerDb";
$wgEnableUserEmail = true;
$wgEmailAuthentication = true;

## Notifications, turned off as we don't send mail
$wgEnotifUserTalk = false;
$wgEnotifWatchlist = false;

## Files
$wgEnableUploads = true;
$wgAllowCopyUploads = true;
$wgUseInstantCommons = false;

## Keys
$wgUpgradeKey = "0j90sa0fjsa90jf0ajfaa";
$wgSecretKey = "j8093j903j902jfr9j109j109jf1093jf09j190jfj09fj1jfknnccnmxnmx";

## PHP Location
// TODO check me
$wgPhpCli = '/usr/local/bin/php';

################################
# MWDD END
################################

// TODO add auto loading of other LocalSetting.php files from a directory based on dbname -> file name...