<?php
// Swift storage is opt-in per wiki. Starting a service never changes storage.

function wmcliSwiftBackendConfig( string $name, string $wikiId, int $shardLevels = 0 ): array {
	$shards = array_fill_keys( [ 'local-public', 'local-thumb', 'local-temp', 'local-transcoded' ],
		[ 'levels' => $shardLevels, 'base' => 16, 'repeat' => 1 ] );
	$shards['local-deleted'] = [ 'levels' => $shardLevels, 'base' => 36, 'repeat' => 0 ];
	return [
		'class' => Wikimedia\FileBackend\SwiftFileBackend::class,
		'name' => $name,
		'wikiId' => $wikiId,
		'lockManager' => 'fsLockManager',
		'swiftAuthUrl' => 'http://swift:8080/auth',
		'swiftStorageUrl' => 'http://swift:8080/v1/AUTH_mw',
		'swiftUser' => 'mw:media',
		'swiftKey' => getenv( 'SWIFT_MEDIA_KEY' ),
		'swiftTempUrlKey' => getenv( 'SWIFT_TEMP_URL_KEY' ),
		'shardViaHashLevels' => $shards,
		'parallelize' => 'implicit',
		'readUsers' => [ 'mw:thumbor' ],
		'writeUsers' => [ 'mw:thumbor' ],
		'secureReadUsers' => [ 'mw:thumbor-private' ],
		'secureWriteUsers' => [ 'mw:thumbor-private' ],
		'canShellboxGetTempUrl' => true,
	];
}

$swiftWikis = array_filter( array_map( 'trim', explode( ',', getenv( 'MEDIAWIKI_SWIFT_WIKIS' ) ?: '' ) ) );
if ( in_array( $wgDBname, $swiftWikis, true ) ) {
	$privateWikis = array_map( 'trim', explode( ',', getenv( 'MEDIAWIKI_SWIFT_PRIVATE_WIKIS' ) ?: '' ) );
	$privateFiles = in_array( $wgDBname, $privateWikis, true );
	$backend = "wmcli-swift-$wgDBname";
	$wgFileBackends[] = wmcliSwiftBackendConfig( $backend, "wikipedia-$wgDBname" );
	$uploadOrigin = rtrim( getenv( 'MEDIAWIKI_SWIFT_PUBLIC_URL' ) ?: '/proxy/swift', '/' );
	$wgUploadPath = $privateFiles ? "$wgScriptPath/img_auth.php" : "$uploadOrigin/wikipedia/$wgDBname";
	$wgLocalFileRepo = [
		'class' => MediaWiki\FileRepo\LocalRepo::class,
		'name' => 'local',
		'backend' => $backend,
		'url' => $wgUploadPath,
		'scriptDirUrl' => $wgScriptPath,
		'hashLevels' => 2,
		'deletedHashLevels' => 3,
		'isPrivate' => $privateFiles,
	];
	$thumborWikis = array_map( 'trim', explode( ',', getenv( 'MEDIAWIKI_THUMBOR_WIKIS' ) ?: '' ) );
	if ( in_array( $wgDBname, $thumborWikis, true ) ) {
		$wgLocalFileRepo += [
			'disableLocalTransform' => true,
			'transformVia404' => true,
			'useJsonMetadata' => true,
			'thumbProxyUrl' => "http://thumbor:8800/wikipedia/$wgDBname/thumb/",
			'thumbProxySecret' => getenv( 'THUMBOR_PRIVATE_SECRET' ),
		];
		if ( $privateFiles ) {
			$wgLocalFileRepo['zones']['thumb']['url'] = "$wgScriptPath/thumb_handler.php";
		}
	}
}
