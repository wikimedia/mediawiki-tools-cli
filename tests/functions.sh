#!/bin/bash

NC='\033[0m' # No Color
RED='\033[0;31m'
GREEN='\033[0;32m'

test_file_contains() {
    local file="$1"
    local expected_match="$2"

    if ! grep -q "$expected_match" "$file"; then
        echo -e "${RED}FAIL:${NC} file did not contain \"$expected_match\""
        echo "File content was..."
        cat "$file"
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
        return 1
    fi
}