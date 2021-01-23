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
if ( PHP_SAPI === 'cli' && !defined( 'MW_DB' ) ) {
    define( 'MW_DB', 'default' );
}

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
	'db-replica' => gethostbyname('db-replica') !== 'db-replica' && !defined( 'MW_PHPUNIT_TEST' ),
	'redis' => gethostbyname('redis') !== 'redis' && !defined( 'MW_PHPUNIT_TEST' ),
	'graphite-statsd' => gethostbyname('graphite-statsd') !== 'graphite-statsd' && !defined( 'MW_PHPUNIT_TEST' ),
];

################################
# MWDD Database
################################
$wgDBname = $dockerDb;

$wgDBservers = [
	[
		'host' => "db-master",
		'dbname' => $dockerDb,
		'user' => 'root',
		'password' => 'toor',
		'type' => "mysql",
		'flags' => DBO_DEFAULT,
		'load' => $mwddServices['db-replica'] ? 0 : 1,
	],
];
if($mwddServices['db-replica'] ) {
	$wgDBservers[] = [
		'host' => "db-replica",
		'dbname' => $dockerDb,
		'user' => 'root',
		'password' => 'toor',
		'type' => "mysql",
		'flags' => DBO_DEFAULT,
		# Avoid switching to readonly too early (for example during update.php)
		'max lag' => 60,
		'load' => 1,
	];
}

// mysql only stuff (would need to change for sqlite?)
$wgDBprefix = "";
$wgDBTableOptions = "ENGINE=InnoDB, DEFAULT CHARSET=binary";

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
## Settings added over time
$wgShowHostnames = true;
$wgAssumeProxiesUseDefaultProtocolPorts = false;

## Site settings
$wgScriptPath = "";
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
$dockerLogDirectory = "/var/log/mediawiki"
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