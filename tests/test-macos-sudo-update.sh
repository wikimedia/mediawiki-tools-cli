#!/usr/bin/env bash
# T384914: sudo update must not make the invoking user's config root-owned.
set -euo pipefail

if [[ "$(uname -s)" != Darwin || "$(id -u)" == 0 ]]; then
    echo "This test requires an unprivileged macOS user with passwordless sudo"
    exit 1
fi
sudo -n true
export MWCLI_CONTEXT_TEST=1

TEST_ROOT="$(mktemp -d)"
trap 'sudo -n rm -rf "$TEST_ROOT"' EXIT
REPLACEMENT="$TEST_ROOT/update artifact/mw"
mkdir -p "$(dirname "$REPLACEMENT")"
go build -ldflags='-X main.GitCommit=macos-sudo-update-fixture' -o "$REPLACEMENT" .
EXPECTED_HASH="$(shasum -a 256 "$REPLACEMENT" | cut -d ' ' -f 1)"
ORIGINAL_HASH="$(shasum -a 256 ./bin/mw | cut -d ' ' -f 1)"
[[ "$EXPECTED_HASH" != "$ORIGINAL_HASH" ]]

failures=0
for scenario in fresh-config existing-config; do
    echo "Checking sudo update with $scenario"
    case_root="$TEST_ROOT/$scenario"
    mkdir -p "$case_root/installed cli" "$case_root/home/Library/Application Support"
    installed="$case_root/installed cli/mw"
    cp ./bin/mw "$installed"
    chmod 755 "$installed"
    sudo -n chown -R root:wheel "$case_root/installed cli"
    case_home="$case_root/home"
    config_home="$case_home/Library/Application Support"
    user_mw() {
        env HOME="$case_home" XDG_CONFIG_HOME="$config_home" \
            MWCLI_CONTEXT_TEST=1 "$installed" "$@"
    }
    config_dir="$config_home/mwcli"
    config_file="$config_dir/config.json"
    before_hash=""
    if [[ "$scenario" == existing-config ]]; then
        user_mw --no-interaction config set gerrit.username before-update
        before_hash="$(shasum -a 256 "$config_file" | cut -d ' ' -f 1)"
    else
        [[ ! -e "$config_dir" ]]
    fi

    # Preserve the user's config location explicitly across sudo. This matches
    # the reported /Users/.../Library/Application Support/mwcli/config.json path
    # regardless of the runner's sudo HOME-reset policy.
    sudo -n env HOME="$case_home" XDG_CONFIG_HOME="$config_home" \
        MWCLI_CONTEXT_TEST=1 "$installed" --no-interaction update --version "$REPLACEMENT"
    [[ "$(shasum -a 256 "$installed" | cut -d ' ' -f 1)" == "$EXPECTED_HASH" ]]
    echo "PASS: sudo replaced the root-owned executable ($scenario)"

    if [[ "$scenario" == fresh-config && -e "$config_dir" ]]; then
        echo "FAIL: elevated update created user configuration on first use"
        sudo -n ls -ld "$config_dir" "$config_file"
        failures=$((failures + 1))
    elif [[ "$scenario" == existing-config ]]; then
        if [[ "$(sudo -n shasum -a 256 "$config_file" | cut -d ' ' -f 1)" != "$before_hash" ]]; then
            echo "FAIL: elevated update changed existing user configuration"
            failures=$((failures + 1))
        fi
    fi
    if ! user_mw --no-interaction config show; then
        echo "FAIL: normal user cannot read config after sudo update ($scenario)"
        failures=$((failures + 1))
        continue
    fi
    if [[ "$scenario" == existing-config ]]; then
        [[ "$(user_mw --no-interaction config get gerrit.username)" == before-update ]]
    fi
    user_mw --no-interaction config set gerrit.username after-update
    [[ "$(user_mw --no-interaction config get gerrit.username)" == after-update ]]
    [[ "$(stat -f '%u' "$config_dir")" == "$(id -u)" ]]
    [[ "$(stat -f '%u' "$config_file")" == "$(id -u)" ]]
    echo "PASS: user-owned config remains readable and writable ($scenario)"
done

if [[ "$failures" != 0 ]]; then
    echo "FAIL: $failures sudo-update configuration checks failed (T384914)"
    exit 1
fi
echo "PASS: sudo update preserves user configuration on first and subsequent use"
