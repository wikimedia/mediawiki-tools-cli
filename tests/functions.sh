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

test_gateway_auth_args() {
	if [ -z "${MWCLI_TEST_GATEWAY_PASSWORD:-}" ] || [ -z "${MWCLI_TEST_GATEWAY_USER:-}" ]; then
		local credentials
		credentials=$("$SCRIPT_DIR/../bin/mw" dev traefik credentials)
		MWCLI_TEST_GATEWAY_PASSWORD=$(echo "$credentials" | sed -n 's/^Password: //p' | tail -n 1)
		MWCLI_TEST_GATEWAY_USER=$(echo "$credentials" | sed -n 's/^Username: //p' | tail -n 1)
	fi
	if [ -z "${MWCLI_TEST_GATEWAY_PASSWORD:-}" ] || [ -z "${MWCLI_TEST_GATEWAY_USER:-}" ]; then
		echo "Failed to read the generated Traefik test credentials"
		return 1
	fi
}

test_wget() {
    local url="$1"
    local expected_match="$2"
    test_gateway_auth_args
    set +e
    local WGET
    WGET=$(curl -fsSL --user "$MWCLI_TEST_GATEWAY_USER:$MWCLI_TEST_GATEWAY_PASSWORD" "$url")
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

test_http_status() {
    local url="$1"
    local expected_status="$2"
    local use_gateway_auth="${3:-0}"
    local host_header="${4:-}"
    local status

    if [ "$use_gateway_auth" = "1" ]; then
        test_gateway_auth_args
        if [ -n "$host_header" ]; then
            status=$(curl -sS -H "Host: $host_header" -u "$MWCLI_TEST_GATEWAY_USER:$MWCLI_TEST_GATEWAY_PASSWORD" -o /dev/null -w "%{http_code}" "$url")
        else
            status=$(curl -sS -u "$MWCLI_TEST_GATEWAY_USER:$MWCLI_TEST_GATEWAY_PASSWORD" -o /dev/null -w "%{http_code}" "$url")
        fi
    elif [ -n "$host_header" ]; then
        status=$(curl -sS -H "Host: $host_header" -o /dev/null -w "%{http_code}" "$url")
    else
        status=$(curl -sS -o /dev/null -w "%{http_code}" "$url")
    fi

    if [ "$status" = "$expected_status" ]; then
        echo -e "${GREEN}PASS:${NC} $url returned HTTP $status"
    else
        echo -e "${RED}FAIL:${NC} $url returned HTTP $status, expected $expected_status"
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            exit 1
        fi
        return 1
    fi
}

# Start the profile-gated ForwardAuth service and wait until Traefik can reach
# it. This lets local integration tests exercise protected path routes without
# depending on an external tunnel provider.
test_start_gateway_auth() {
    local probe_url="$1"
    local status=""

    test_command_success ./bin/mw dev compose up -- -d gateway-auth
    for ((attempt=1; attempt<=60; attempt++)); do
        status=$(curl --silent --max-time 3 -o /dev/null -w '%{http_code}' "$probe_url" || true)
        if [ "$status" = "401" ]; then
            echo -e "${GREEN}PASS:${NC} local gateway authentication is ready"
            return 0
        fi
        sleep 1
    done

    echo -e "${RED}FAIL:${NC} local gateway authentication did not become ready (last HTTP status: ${status:-none})"
    if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
        exit 1
    fi
    return 1
}

# Exercise the real Traefik ForwardAuth middleware with the same HTTP upstream
# and HTTPS forwarded origin that a TLS-terminating tunnel uses.
test_gateway_https_session() (
    set -e
    local url="$1"
    local work
    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT
    test_gateway_auth_args
    local headers=(-H 'Host: gateway-test.example.test' -H 'X-Forwarded-Proto: https')
    curl --connect-timeout 2 --max-time 10 -fsS "${headers[@]}" -D "$work/headers" -o "$work/form" "$url/_wm/login"
    local csrf cookie status
    csrf=$(sed -n 's/.*name="csrf" value="\([^"]*\)".*/\1/p' "$work/form")
    cookie=$(sed -n 's/^[Ss]et-[Cc]ookie: \([^;]*\);.*/\1/p' "$work/headers")
    test -n "$csrf"
    status=$(curl --connect-timeout 2 --max-time 10 -sS "${headers[@]}" -H "Cookie: $cookie" \
        --data-urlencode "username=$MWCLI_TEST_GATEWAY_USER" \
        --data-urlencode "password=$MWCLI_TEST_GATEWAY_PASSWORD" \
        --data-urlencode "csrf=$csrf" --data-urlencode 'return=/proxy/adminer/' \
        -D "$work/headers" -o /dev/null -w '%{http_code}' "$url/_wm/login")
    if [ "$status" != 303 ]; then
        echo -e "${RED}FAIL:${NC} HTTPS gateway login returned $status, expected 303"
        exit 1
    fi
    cookie=$(sed -n 's/^[Ss]et-[Cc]ookie: \([^;]*_session=[^;]*\);.*/\1/p' "$work/headers")
    test -n "$cookie"
    grep -q '; Secure' "$work/headers"
    # No gateway Basic credentials: the HTTPS session alone must pass ForwardAuth,
    # even when an application supplies its own Authorization header.
    status=$(curl --connect-timeout 2 --max-time 10 -sS "${headers[@]}" -H "Cookie: $cookie" -H 'Authorization: Bearer application-token' \
        -o /dev/null -w '%{http_code}' "$url/proxy/adminer/")
    if [ "$status" != 200 ]; then
        echo -e "${RED}FAIL:${NC} HTTPS gateway session returned $status, expected 200"
        exit 1
    fi
    status=$(curl --connect-timeout 2 --max-time 10 -sS -H 'Host: gateway-test.example.test' -H "Cookie: $cookie" \
        -o /dev/null -w '%{http_code}' "$url/proxy/adminer/")
    if [ "$status" != 401 ]; then
        echo -e "${RED}FAIL:${NC} HTTPS session on an HTTP origin returned $status, expected 401"
        exit 1
    fi
    echo -e "${GREEN}PASS:${NC} HTTPS gateway session survives ForwardAuth and remains bound to its origin"
)

test_wget_eventually_contains() {
    local url="$1"
    local expected_match="$2"
    local max_attempts="${3:-30}"
    local sleep_seconds="${4:-1}"

    local attempt=1
    local last_status=""
    local last_body=""
    local last_curl_error=""
    test_gateway_auth_args

    while [ "$attempt" -le "$max_attempts" ]; do
        set +e
        local response
        response=$(curl -sS -L --connect-timeout 2 --max-time 5 -u "$MWCLI_TEST_GATEWAY_USER:$MWCLI_TEST_GATEWAY_PASSWORD" -w "\n%{http_code}" "$url" 2>&1)
        local curl_result=$?
        set -e

        local status
        status=$(echo "$response" | tail -n 1)
        local body
        body=$(echo "$response" | sed '$d')

        if [ "$curl_result" -eq 0 ]; then
            last_status="$status"
            last_body="$body"

            if echo "$body" | grep -q "$expected_match"; then
                echo -e "${GREEN}PASS:${NC} $url returned expected content on attempt $attempt/$max_attempts (HTTP $status)"
                return 0
            fi
        else
            last_status="$status"
            last_body="$body"
            last_curl_error="$body"
        fi

        if [ "$attempt" -lt "$max_attempts" ]; then
            sleep "$sleep_seconds"
        fi
        attempt=$((attempt + 1))
    done

    echo -e "${RED}FAIL:${NC} $url did not contain \"$expected_match\" after $max_attempts attempts"
    if [ -n "$last_status" ]; then
        echo "Last HTTP status: $last_status"
    fi
    if [ -n "$last_curl_error" ]; then
        echo "Last curl error: $last_curl_error"
    fi

    if [ -n "$last_body" ]; then
        echo "Last response body was..."
        echo "$last_body"
    fi

    if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
        echo "STOP_ON_FAIL=1 detected, exiting immediately"
        exit 1
    fi
    return 1
}

test_wget_eventually_200() {
    local url="$1"
    local max_attempts="${2:-5}"
    local sleep_seconds="${3:-0.5}"

    local attempt=1
    local last_status=""
	test_gateway_auth_args

    while [ "$attempt" -le "$max_attempts" ]; do
        set +e
        local status
        status=$(curl -sS -L -u "$MWCLI_TEST_GATEWAY_USER:$MWCLI_TEST_GATEWAY_PASSWORD" -o /dev/null -w "%{http_code}" "$url")
        local curl_result=$?
        set -e

        if [ "$curl_result" -eq 0 ]; then
            last_status="$status"
            if [ "$status" = "200" ]; then
                echo -e "${GREEN}PASS:${NC} $url returned HTTP 200 on attempt $attempt/$max_attempts"
                return 0
            fi
        fi

        if [ "$attempt" -lt "$max_attempts" ]; then
            sleep "$sleep_seconds"
        fi
        attempt=$((attempt + 1))
    done

    echo -e "${RED}FAIL:${NC} $url did not return HTTP 200 after $max_attempts attempts"
    if [ -n "$last_status" ]; then
        echo "Last HTTP status: $last_status"
    fi
    if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
        echo "STOP_ON_FAIL=1 detected, exiting immediately"
        exit 1
    fi
    return 1
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

test_command_success_or_matching_failure() {
    local expected_failure="$1"
    shift
    set +e
    local OUTPUT
    OUTPUT="$("$@" 2>&1)"
    local RESULT=$?
    set -e
    if [ "$RESULT" -eq 0 ]; then
        echo "$OUTPUT"
        echo -e "${GREEN}PASS:${NC} $* returned $RESULT"
    elif echo "$OUTPUT" | grep -Fq "$expected_failure"; then
        echo -e "${GREEN}SKIP:${NC} $* failed with expected unavailable-service response: $expected_failure"
    else
        echo -e "${RED}FAIL:${NC} $* returned non-zero code $RESULT"
        echo "Raw output was..."
        echo "$OUTPUT"
        if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
            echo "STOP_ON_FAIL=1 detected, exiting immediately"
            exit 1
        fi
        return 1
    fi
}

test_command_success_with_retries() {
    local max_attempts="$1"
    local sleep_seconds="$2"
    shift 2

    local attempt=1
    local output=""
    local result=1
    while [ "$attempt" -le "$max_attempts" ]; do
        set +e
        output="$("$@" 2>&1)"
        result=$?
        set -e

        if [ "$result" -eq 0 ]; then
            echo "$output"
            echo -e "${GREEN}PASS:${NC} $* returned $result on attempt $attempt/$max_attempts"
            return 0
        fi

        if [ "$attempt" -lt "$max_attempts" ]; then
            echo "Command failed with code $result; retrying in ${sleep_seconds}s (attempt $attempt/$max_attempts): $*"
            sleep "$sleep_seconds"
        fi
        attempt=$((attempt + 1))
    done

    echo "$output"
    echo -e "${RED}FAIL:${NC} $* returned non-zero code $result after $max_attempts attempts"
    if [ "${STOP_ON_FAIL:-0}" = "1" ]; then
        echo "STOP_ON_FAIL=1 detected, exiting immediately"
        exit 1
    fi
    return 1
}

test_docker_ps_service_count() {
    local expected_count="$1"
    local context="default"
    if [ -z "${GITLAB_CI:-}" ] && [ -n "${MWCLI_CONTEXT_TEST:-}" ]; then
        context="test"
    elif [ -n "${CONTEXT:-}" ]; then
        context="${CONTEXT}"
    fi

    set +e
    local COUNT
    COUNT="$(docker ps --format '{{.Names}}' | grep -c "^mwcli-mwdd-${context}-")"
    set -e
    if [ "$COUNT" -eq "$expected_count" ]; then
        echo -e "${GREEN}PASS:${NC} docker context $context has $expected_count containers"
    else
        echo -e "${RED}FAIL:${NC} docker context $context has $COUNT containers, expected $expected_count"
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

# Issue simultaneous requests without retries, so transient HTTP failures fail the test.
test_wget_parallel() {
    local url="$1"
    local expected_match="$2"
    local count="${3:-8}"
    local response_dir
    response_dir=$(mktemp -d)
    local pids=()
    local request
    for ((request = 0; request < count; request++)); do
        (
            local response="$response_dir/$request"
            if ! curl --fail --silent --show-error --max-time 60 "$url" -o "$response" ||
                ! grep -Fq -- "$expected_match" "$response"; then
                echo "FAIL: parallel request $request to $url"
                cat "$response" 2>/dev/null || true
                exit 1
            fi
        ) &
        pids+=("$!")
    done
    local failed=0
    local pid
    for pid in "${pids[@]}"; do
        if ! wait "$pid"; then
            failed=1
        fi
    done
    rm -rf "$response_dir"
    if [ "$failed" -eq 0 ]; then
        echo "PASS: $count parallel requests to $url contain $expected_match"
    fi
    return "$failed"
}
