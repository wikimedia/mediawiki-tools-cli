#!/bin/bash

NC='\033[0m' # No Color
RED='\033[0;31m'
GREEN='\033[0;32m'

# Handle STOP_ON_FAIL environment variable
# STOP_ON_FAIL=1 - stop on first failure instead of continuing
# NO_FINISH=1 - prevents the finish function from doing anything
# Note: FINISH=1 is handled after all sourcing completes (see _handle_finish_if_needed)

test_file_contains() {
    local file="$1"
    local expected_match="$2"

    if ! grep -q "$expected_match" "$file"; then
        echo -e "${RED}FAIL:${NC} file did not contain \"$expected_match\""
        echo "File content was..."
        cat "$file"
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            echo "STOP_ON_FAIL=1 detected, exiting immediately"
            exit 1
        fi
        return 1
    else
        echo -e "${GREEN}PASS:${NC} file contained \"$expected_match\""
    fi
}

test_wget() {
    local url="$1"
    local expected_match="$2"
    set +e
    local WGET
    WGET=$(wget -qO- "$url")
    echo "$WGET" | grep -q "$expected_match"
    local RESULT=$?
    set -e
    if [ $RESULT -eq 0 ]; then
        echo -e "${GREEN}PASS:${NC} $url contains \"$expected_match\""
    else
        echo -e "${RED}FAIL:${NC} $url does not contain \"$expected_match\""
        echo "Raw response was..."
        echo "$WGET"
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            echo "STOP_ON_FAIL=1 detected, exiting immediately"
            exit 1
        fi
        return 1
    fi
}

test_command() {
    # Last argument is the expected match
    local expected_match="${*: -1}"
    local command=("${@:1:(($#-1))}")
    set +e
    local OUTPUT
    OUTPUT="$("${command[@]}" 2>&1)"
    echo "$OUTPUT" | grep -q "$expected_match"
    local RESULT=$?
    set -e
    if [ $RESULT -eq 0 ]; then
        echo -e "${GREEN}PASS:${NC} ${command[*]} output contains \"$expected_match\""
    else
        echo -e "${RED}FAIL:${NC} ${command[*]} output does not contain \"$expected_match\""
        echo "Raw output was..."
        echo "$OUTPUT"
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            echo "STOP_ON_FAIL=1 detected, exiting immediately"
            exit 1
        fi
        return 1
    fi
}

test_command_success() {
    set +e
    local OUTPUT
    OUTPUT="$("$@" 2>&1)"
    local RESULT=$?
    set -e
    echo "$OUTPUT"
    if [ $RESULT -eq 0 ]; then
        echo -e "${GREEN}PASS:${NC} $* returned $RESULT"
    else
        echo -e "${RED}FAIL:${NC} $* returned non-zero code $RESULT"
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            echo "STOP_ON_FAIL=1 detected, exiting immediately"
            exit 1
        fi
        return 1
    fi
}

test_docker_ps_service_count() {
    local expected_count="$1"
    set +e
    local COUNT
    COUNT="$(docker ps | grep -c mwcli)"
    set -e
    if [ "$COUNT" -eq "$expected_count" ]; then
        echo -e "${GREEN}PASS:${NC} docker has $expected_count containers"
    else
        echo -e "${RED}FAIL:${NC} docker has $COUNT containers, expected $expected_count"
        docker ps
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            echo "STOP_ON_FAIL=1 detected, exiting immediately"
            exit 1
        fi
        return 1
    fi
}

# Helper function to handle FINISH=1 after all sourcing is complete
# Call this at the end of your test script before the trap statement
_handle_finish_if_needed() {
    if [ "${FINISH:-0}" = "1" ]; then
        if [ "$(type -t finish)" = "function" ]; then
            echo "FINISH=1 detected, calling finish function..."
            finish
            exit 0
        else
            echo "FINISH=1 set but no finish function defined"
            exit 1
        fi
    fi
}

# Wrapper function for finish that respects NO_FINISH flag
_finish_wrapper() {
    if [ "${NO_FINISH:-0}" = "1" ]; then
        echo "NO_FINISH=1 detected, skipping finish function"
        return 0
    fi
    finish
}