<?php
// Adapted from wmf-claude 003fa611cd670d030a0063986a876960f2ba7214 (MIT; LICENSE).
// Prepare thumbnail containers and secure private storage zones.
use MediaWiki\MainConfigNames;
use MediaWiki\Maintenance\Maintenance;

require_once ( getenv( 'MW_INSTALL_PATH' ) ?: '/var/www/html/w' ) . '/maintenance/Maintenance.php';

class PrepareDevSwift extends Maintenance {
	public function __construct() {
		parent::__construct();
		$this->addDescription( 'Prepare Swift thumbnail containers for the selected wiki.' );
	}

	public function execute() {
		$repo = $this->getServiceContainer()->getRepoGroup()->getLocalRepo();
		$backend = $repo->getBackend();
		if ( !$backend instanceof Wikimedia\FileBackend\SwiftFileBackend ) {
			$this->fatalError( 'The selected wiki does not use Swift storage.' );
		}
		$private = $this->getConfig()->get( MainConfigNames::LocalFileRepo )['isPrivate'] ?? false;
		$access = $private ? [ 'noAccess' => true, 'noListing' => true ] : [];
		$zones = $private ? [ 'public', 'thumb', 'temp', 'transcoded', 'deleted' ] : [ 'thumb' ];
		foreach ( $zones as $zone ) {
			// Preparing the root covers every configured shard, including absent
			// containers. secure() then updates ACLs on existing containers too.
			$params = [ 'dir' => $repo->getZonePath( $zone ) ] + $access;
			if ( !$backend->prepare( $params )->isOK() ) {
				$this->fatalError( "Swift $zone containers could not be prepared." );
			}
			if ( $private && !$backend->secure( $params )->isOK() ) {
				$this->fatalError( "Swift $zone containers could not be secured." );
			}
		}
		$this->output( "Swift thumbnail containers are ready.\n" );
	}
}

return PrepareDevSwift::class;
