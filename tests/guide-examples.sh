#!/usr/bin/env bash
# Shared runtime checks for docs/guides/dev/. Callers supply their isolated CI environment.

test_mysql_dump_roundtrip() {
    local mw="$1" work
    work=$(mktemp -d)
    # Guide: mysql-dumps.md. Keep the fixture separate from installed wiki data.
    test_command_success "$mw" --no-interaction dev compose exec -- -T mysql mariadb -uroot -ptoor --default-character-set=utf8mb4 -e "CREATE DATABASE guide_source CHARACTER SET utf8mb4; CREATE DATABASE guide_restore CHARACTER SET utf8mb4; CREATE TABLE guide_source.sample (id INT PRIMARY KEY, body TEXT); INSERT INTO guide_source.sample VALUES (1, 'café'), (2, 'line one\nline two');"
    test_command_success "$mw" --no-interaction dev compose exec -- -T mysql sh -c 'mariadb-dump -uroot -ptoor --single-transaction guide_source > /tmp/wm-guide.sql'
    test_command_success "$mw" --no-interaction dev compose cp mysql:/tmp/wm-guide.sql "$work/source.sql"
    test_command_success "$mw" --no-interaction dev compose exec -- -T mysql mariadb -uroot -ptoor guide_restore < "$work/source.sql"
    test_command "$mw" --no-interaction dev compose exec -- -T mysql mariadb -uroot -ptoor -Nse "SELECT IF((SELECT GROUP_CONCAT(HEX(body) ORDER BY id) FROM guide_source.sample) = (SELECT GROUP_CONCAT(HEX(body) ORDER BY id) FROM guide_restore.sample), 'dump-roundtrip-ok', 'dump-roundtrip-failed')" "dump-roundtrip-ok"
    test_command_success "$mw" --no-interaction dev compose exec -- -T mysql mariadb -uroot -ptoor -e 'DROP DATABASE guide_source; DROP DATABASE guide_restore;'
    test_command_success "$mw" --no-interaction dev compose exec -- -T mysql rm /tmp/wm-guide.sql
    rm -rf "$work"
}

test_xdebug_settings() {
    local mw="$1"
    # Guide: xdebug.md. Verify runtime settings after recreation, not just Compose output.
    test_command "$mw" dev mediawiki exec -- php --ri xdebug 'xdebug.client_host.*xdebug://gateway'
    test_command_success "$mw" dev env set MEDIAWIKI_XDEBUG_MODE off
    test_command_success "$mw" dev mediawiki create --force-recreate
    test_command_success "$mw" dev mediawiki exec -- php -r 'exit(xdebug_info("mode") === [] ? 0 : 1);'
    test_command_success "$mw" dev env set MEDIAWIKI_XDEBUG_MODE debug
    test_command_success "$mw" dev mediawiki create --force-recreate
    test_command_success "$mw" dev mediawiki exec -- php -r 'exit(in_array("debug", xdebug_info("mode"), true) ? 0 : 1);'
    test_command "$mw" dev mediawiki exec -- PHP_IDE_CONFIG=serverName=guide php -r 'echo getenv("PHP_IDE_CONFIG");' 'serverName=guide'
}

test_http_port_change() {
    local mw="$1" new_port="$2"
    # Guide: ports.md. Existing containers must acquire the new binding and dashboard settings.
    test_command_success "$mw" dev env set PORT "$new_port"
    test_command_success "$mw" dev compose up -- -d --force-recreate traefik dashboard
    test_command "$mw" dev compose port traefik "$new_port" ":$new_port"
    test_command "$mw" dev compose exec -- -T dashboard printenv PORT "$new_port"
}

test_fresh_recording() {
    local mw="$1" output_dir video container_video found=0
    # Guide: fresh.md. Keep reports under the mounted checkout and require a playable recording.
    mkdir -p tests/selenium/log
    output_dir=$(mktemp -d "$(pwd)/tests/selenium/log/guide-recording.XXXXXX")
    test_command_success "$mw" dev mediawiki fresh "LOG_DIR=/var/www/html/w/tests/selenium/log/$(basename "$output_dir")" npm run selenium-test -- -- --recordVideo --useBrowserHeadless=false --maxInstances=1 --spec tests/selenium/specs/recentchanges.js
    for video in "$output_dir"/*.mp4; do
        [ -s "$video" ] || continue
        container_video="/var/www/html/w/tests/selenium/log/$(basename "$output_dir")/$(basename "$video")"
        test_command_success_with_retries 10 1 "$mw" dev mediawiki fresh -- sh -c 'ffprobe -v error -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 "$1" | grep -qx h264' sh "$container_video"
        found=1
    done
    if [ "$found" -ne 1 ]; then
        echo "FAIL: Fresh recording produced no nonempty MP4 in $output_dir"
        return 1
    fi
    echo "PASS: Fresh produced a playable recording in $output_dir"
}
