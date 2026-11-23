#!/usr/bin/env bash
# Requires an installed default wiki in the caller's disposable development context.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/functions.sh"
MW_BIN=$(test_cli_binary "${MW_BIN:-}")

flavour=
cleanup() {
    local result=$?
    if [[ -n "$flavour" ]]; then
        if ((result != 0)); then
            "$MW_BIN" dev compose logs -- --tail 100 "shellbox-$flavour" "shellbox-$flavour-httpd" || true
        fi
        "$MW_BIN" --no-interaction dev shellbox "$flavour" destroy || result=1
    fi
    exit "$result"
}
trap cleanup EXIT

# eval.php evaluates one line at a time and can report an exception without
# exiting nonzero. Require a success marker after every remote request passed.
probe=$(awk '{ printf "%s ", $0 } END { print "" }' <<'PHP'
$services = MediaWiki\MediaWikiServices::getInstance();
$clients = $services->getShellboxClientFactory();
$commands = $services->getShellCommandFactory();
$marker = getenv( 'MWCLI_SHELLBOX_MARKER' );
foreach ( explode( ',', getenv( 'MWCLI_SHELLBOX_SERVICES' ) ) as $name ) {
    if ( !$clients->isEnabled( $name ) ) {
        throw new RuntimeException( "Remote Shellbox is disabled for $name" );
    }
    $result = $commands->createBoxed( $name, 5 )
        ->routeName( 'wmcli-integration' )
        ->params( '/bin/echo', $marker )
        ->execute();
    if ( $result->getExitCode() !== 0 || trim( $result->getStdout() ) !== $marker ) {
        throw new RuntimeException( "Unexpected Shellbox result for $name" );
    }
}
echo "$marker passed\n";
PHP
)

probe_shellbox() {
    local output
    output=$("$MW_BIN" dev compose exec -- -T \
        -e "MWCLI_SHELLBOX_SERVICES=$clients" -e "MWCLI_SHELLBOX_MARKER=$marker" \
        mediawiki php maintenance/run.php eval --wiki default <<< "$probe") || return 1
    printf '%s\n' "$output"
    grep -Fxq "$marker passed" <<< "$output"
}

echo '=== Shellbox: authenticated remote execution ==='
for flavour in media php-rpc score syntaxhighlight timeline; do
    case "$flavour" in
        media) clients=djvu,pagedtiffhandler,pdfhandler,timedmediahandler ;;
        php-rpc) clients=constraint-regex-checker ;;
        score) clients=score ;;
        syntaxhighlight) clients=syntaxhighlight ;;
        timeline) clients=easytimeline ;;
    esac
    marker="shellbox-$flavour-$$-$RANDOM"
    echo "=== Shellbox $flavour: create, request, destroy ==="
    test_command_success "$MW_BIN" --no-interaction dev shellbox "$flavour" create
    # Container startup and dashboard service discovery are asynchronous.
    test_command_success_with_retries 15 1 probe_shellbox
    test_command_success "$MW_BIN" --no-interaction dev shellbox "$flavour" destroy
    flavour=
done
echo '=== Shellbox remote execution passed; all five flavours removed ==='
