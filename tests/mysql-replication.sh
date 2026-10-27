#!/usr/bin/env bash

# Source after functions.sh. Requires an existing primary and configured replica.
# Optional arguments after the binary are dev flags, e.g. --context NAME.
test_mysql_replication_lifecycle() (
    set -e
    local mw="$1"
    shift
    local dev=("$mw" --no-interaction dev "$@")
    local query="SELECT GROUP_CONCAT(id ORDER BY id) FROM mwcli_replication_test.lifecycle"
    local primary_id replica_id

    container_ids() {
        primary_id=$("${dev[@]}" compose ps -- --quiet mysql)
        replica_id=$("${dev[@]}" compose ps -- --quiet mysql-replica)
        [ -n "$primary_id" ] && [ -n "$replica_id" ]
    }

    sql() {
        local container_id="$primary_id"
        if [ "$1" = mysql-replica ]; then container_id="$replica_id"; fi
        docker exec "$container_id" mariadb --host=127.0.0.1 --connect-timeout=5 -uroot -ptoor --batch --skip-column-names -e "$2"
    }

    replica_status() {
        docker exec "$replica_id" mariadb --host=127.0.0.1 --connect-timeout=5 -uroot -ptoor --batch -e 'SHOW SLAVE STATUS\G'
    }

    diagnostics() {
        replica_status || true
        "${dev[@]}" compose logs -- --tail 80 mysql mysql-replica mysql-configure-replication mysql-replica-configure-replication || true
    }

    wait_for_sql() {
        local service="$1" statement="$2" expected="$3" description="$4"
        local attempt output=""
        for ((attempt=1; attempt<=60; attempt++)); do
            if output=$(sql "$service" "$statement" 2>&1) && [ "$output" = "$expected" ]; then
                echo "PASS: $description"
                return 0
            fi
            sleep 1
        done
        echo "FAIL: $description; expected '$expected', last SQL response: $output"
        diagnostics
        return 1
    }

    wait_for_replica_threads() {
        local attempt status=""
        for ((attempt=1; attempt<=60; attempt++)); do
            if status=$(replica_status 2>&1) &&
                echo "$status" | grep -Eq '^[[:space:]]*Slave_IO_Running: Yes$' &&
                echo "$status" | grep -Eq '^[[:space:]]*Slave_SQL_Running: Yes$'; then
                echo 'PASS: replica IO and SQL threads are running'
                return 0
            fi
            sleep 1
        done
        echo "FAIL: replica threads did not become healthy: $status"
        diagnostics
        return 1
    }

    wait_for_replication() {
        wait_for_sql mysql-replica "$query" "$1" "$2"
        wait_for_replica_threads
    }

    container_ids
    wait_for_sql mysql 'SELECT 1' 1 'primary accepts SQL'
    # Bootstrap records a starting binlog position. Do not write the fixture
    # until setup finishes, otherwise it could precede that position.
    wait_for_replica_threads
    test_command_success sql mysql 'CREATE DATABASE mwcli_replication_test; CREATE TABLE mwcli_replication_test.lifecycle (id INT PRIMARY KEY); INSERT INTO mwcli_replication_test.lifecycle VALUES (1);'
    wait_for_replication 1 'initial write reaches replica'

    # Writes made while the replica is offline must be caught up on startup.
    test_command_success "${dev[@]}" mysql-replica stop
    test_command_success sql mysql 'INSERT INTO mwcli_replication_test.lifecycle VALUES (2);'
    test_command_success "${dev[@]}" mysql-replica start
    container_ids
    wait_for_replication 1,2 'replica catches up after being stopped'

    # Use the public environment lifecycle, including its saved resume set.
    test_command_success "${dev[@]}" stop
    local running
    running=$("${dev[@]}" compose ps -- --status running --quiet)
    if [ -n "$running" ]; then
        echo "FAIL: containers remain running after dev stop: $running"
        diagnostics
        exit 1
    fi
    echo 'PASS: all environment services stopped'
    test_command_success "${dev[@]}" start
    container_ids
    wait_for_sql mysql "$query" 1,2 'primary data survives environment stop/start'
    wait_for_replication 1,2 'replica data and threads survive environment stop/start'
    test_command_success sql mysql 'INSERT INTO mwcli_replication_test.lifecycle VALUES (3);'
    wait_for_replication 1,2,3 'new writes replicate after environment stop/start'

    test_command_success sql mysql 'DROP DATABASE mwcli_replication_test;'
    wait_for_sql mysql-replica "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='mwcli_replication_test'" 0 'fixture removal replicates'
)
