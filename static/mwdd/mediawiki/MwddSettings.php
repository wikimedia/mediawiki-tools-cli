<?php

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
$mwddServices = [
	'mysql-replica' => gethostbyname('mysql-replica') !== 'mysql-replica' && !defined( 'MW_PHPUNIT_TEST' ),
	'redis' => gethostbyname('redis') !== 'redis' && !defined( 'MW_PHPUNIT_TEST' ),
	'graphite-statsd' => gethostbyname('graphite-statsd') !== 'graphite-statsd' && !defined( 'MW_PHPUNIT_TEST' ),
];

################################
# MWDD Database
################################
// Figure out if we are using sqlite, or if this should be mysql..?
if( file_exists( $IP . '/data/' . $dockerDb . '.sqlite' ) ) {
	$dockerDbType = 'sqlite';
} else {
	// TODO cache this check somehow so that we don't need a query every time...
	try{
		$mysqlPdo = new PDO( "mysql:host=mysql;dbname=" . $dockerDb, 'root', 'toor' );
		$mysqlCheck = $mysqlPdo->query("SHOW DATABASES LIKE " . $dockerDb);
		if(count( $mysqlCheck ) === 1){
			$dockerDbType = 'mysql';
		}
	} catch ( Exception $e ) {
		// do nothing
	}
	// If no other magic detection happened, we must be in postgres (or some generic error state)
	if(!isset($dockerDbType)){
		$dockerDbType = 'postgres';
	}
}

$wgDBname = $dockerDb;

if( $dockerDbType === 'sqlite' ) {
	$wgDBservers = [
		[
			'dbDirectory' => $IP . '/data',
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
			'load' => $mwddServices['mysql-replica'] ? 0 : 1,
		],
	];
	if($mwddServices['mysql-replica'] ) {
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
# MWDD Redis
################################
if(gethostbyname('redis') !== 'redis') {
	$wgObjectCaches['redis'] = [
		'class' => 'RedisBagOStuff',
		'servers' => [ 'redis:6379' ],
	];
}


################################
# MWDD Graphite & Statsd
################################
if(gethostbyname('graphite-statsd') !== 'graphite-statsd') {
	$wgStatsdServer = "graphite-statsd";
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
$wgTmpDirectory = "{$wgUploadDirectory}/tmp";
$wgCacheDirectory = "{$wgUploadDirectory}/cache";

$wgUploadPath = "{$wgScriptPath}/images/docker/{$dockerDb}";

## Dev & Debug
$dockerLogDirectory = "/var/log/mediawiki";
$wgDebugLogFile = "$dockerLogDirectory/debug.log";

ini_set( 'xdebug.var_display_max_depth', -1 );
ini_set( 'xdebug.var_display_max_children', -1 );
ini_set( 'xdebug.var_display_max_data', -1 );

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