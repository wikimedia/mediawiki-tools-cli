NC='\033[0m' # No Color
RED='\033[0;31m'
GREEN='\033[0;32m'

test_file_contains() {
    local file="$1"
    local expected_match="$2"

    if ! grep -q "$expected_match" "$file"; then
        printf "${RED}FAIL:${NC} file did not contain \"$expected_match\"\n"
        echo "File content was..."
        cat $file
        return 1
    else
        printf "${GREEN}PASS:${NC} file contained \"$expected_match\"\n"
    fi
}

test_wget() {
    url=$1
    expected_match=$2
    set +e
    WGET=$(wget -qO- $url)
    echo "$WGET" | grep -q "$expected_match"
    RESULT=$?
    set -e
    if [ $RESULT -eq 0 ]; then
        printf "${GREEN}PASS:${NC} $url contains \"$expected_match\"\n"
    else
        printf "${RED}FAIL:${NC} $url does not contain \"$expected_match\"\n"
        echo "Raw response was..."
        echo "$WGET"
        return 1
    fi
}

test_command() {
    command=$1
    expected_match=$2
    set +e
    OUTPUT=$($command 2>&1)
    echo "$OUTPUT" | grep -q "$expected_match"
    RESULT=$?
    set -e
    if [ $RESULT -eq 0 ]; then
        printf "${GREEN}PASS:${NC} $command output contains \"$expected_match\"\n"
    else
        printf "${RED}FAIL:${NC} $command output does not contain \"$expected_match\"\n"
        echo "Raw output was..."
        echo "$OUTPUT"
        return 1
    fi
}

test_command_success() {
    command=$@
    set +e
    OUTPUT=$($command)
    RESULT=$?
    set -e
    echo "$OUTPUT"
    if [ $RESULT -eq 0 ]; then
        printf "${GREEN}PASS:${NC} $command returned $RESULT\n"
    else
        printf "${RED}FAIL:${NC} $command returned non-zero code $RESULT\n"
        return 1
    fi
}

test_docker_ps_service_count() {
    expected_count=$1
    set +e
    COUNT=$(docker ps | grep mwcli | wc -l)
    set -e
    if [ $COUNT -eq $expected_count ]; then
        printf "${GREEN}PASS:${NC} docker has $expected_count containers\n"
    else
        printf "${RED}FAIL:${NC} docker has $COUNT containers, expected $expected_count\n"
        docker ps
        return 1
    fi
}