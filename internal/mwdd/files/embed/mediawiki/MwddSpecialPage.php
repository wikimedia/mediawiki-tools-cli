<?php

class MwddSpecial extends SpecialPage {

	public function __construct() {
		parent::__construct( 'Mwdd' );
	}

	/**
	 * @see SpecialPage::execute
	 *
	 * @param string|null $subPage
	 */
	public function execute( $subPage ) {
		parent::execute( $subPage );
		global $mwddServices, $dockerDbType;

		// TODO output db type, dbname & other db settings here
		$this->getOutput()->addHTML( "Which services are running?" );
		$this->getOutput()->addHTML( "</br>" );
		$this->getOutput()->addHTML( json_encode( $mwddServices ) );
		$this->getOutput()->addHTML( "</br>" );
		$this->getOutput()->addHTML( "Which DB is being used" );
		$this->getOutput()->addHTML( "</br>" );
		$this->getOutput()->addHTML( $dockerDbType );
		$this->getOutput()->addHTML( "</br>" );
		$this->getOutput()->addHTML( "How does DB lag look?" );
		$this->getOutput()->addHTML( "</br>" );
		$this->getOutput()->addHTML( json_encode( \MediaWiki\MediaWikiServices::getInstance()->getDBLoadBalancer()->getMaxLag()[1] ) );

	}

}