set -euo pipefail
umask 077
workspace=$1
metadata=$2
source=$3
action=$4
context=${5:-$(basename "$workspace")}
port=${6:-}
if [[ -z $port && -f $metadata/editor-port ]]; then port=$(cat "$metadata/editor-port"); fi
port=${port:-46300}
mkdir -p "$metadata"
exec 6>"$metadata/editor.lock"
flock -w 30 6 || { echo 'Another browser editor operation is still running.' >&2; exit 1; }
pidfile=$metadata/editor.pid
running() {
    test -f "$pidfile" || return 1
    pid=$(cat "$pidfile")
    [[ $pid =~ ^[0-9]+$ ]] || return 1
    kill -0 "$pid" 2>/dev/null || return 1
    test -r "/proc/$pid/cmdline" || return 1
    tr '\0' '\n' < "/proc/$pid/cmdline" | grep -Fqx -- "$metadata/editor-data"
}
stop_editor() {
    if ! running; then return 0; fi
    kill "$pid"
    for ((attempt=0; attempt<200; attempt++)); do
        if ! running; then return 0; fi
        sleep 0.1
    done
    echo 'Browser editor is still stopping; retry before deleting its workspace.' >&2
    return 1
}
if [[ $action == status ]]; then
    if running; then echo 'Browser editor is running (authenticated loopback service).'; else echo 'Browser editor is stopped.'; fi
    exit 0
fi
if [[ $action == stop ]]; then
    stop_editor
    rm -f "$pidfile"
    exit 0
fi
if running && [[ $(cat "$metadata/editor-port") == "$port" ]]; then exit 0; fi
stop_editor
version=4.144.0
case "$(uname -m)" in
    x86_64) arch=amd64; checksum=710f749c40a0f3d0ab3be09c0de65314e5c2907cc50646f5fbc0f24c0e8fba2d ;;
    aarch64) arch=arm64; checksum=4404c784f541bd56a2a8d6e2a9474ed0b4e37bc8502aa3a247898ca933434482 ;;
    *) echo 'The browser editor currently supports Linux amd64 and arm64.' >&2; exit 1 ;;
esac
cache=$HOME/.cache/a-wikimedia-cli/code-server/$version-$arch
mkdir -p "$(dirname "$cache")"
( flock 7; printf '%s\n' "$cache" > "$metadata/editor-cache" ) 7>"$HOME/.cache/a-wikimedia-cli/remote-cache.lock"
if ! test -x "$cache/bin/code-server"; then
    mkdir -p "$(dirname "$cache")"
    exec 8>"$cache.lock"
    flock 8
    if ! test -x "$cache/bin/code-server"; then
        temporary=$(mktemp -d "$(dirname "$cache")/download.XXXXXXXX")
        trap 'rm -rf -- "$temporary"' EXIT
        curl --fail --location --retry 3 --output "$temporary/archive.tar.gz" \
            "https://github.com/coder/code-server/releases/download/v$version/code-server-$version-linux-$arch.tar.gz"
        printf '%s  %s\n' "$checksum" "$temporary/archive.tar.gz" | sha256sum --check --status
        mkdir "$temporary/editor"
        tar -xzf "$temporary/archive.tar.gz" --strip-components=1 -C "$temporary/editor"
        mv "$temporary/editor" "$cache"
        rm -rf -- "$temporary"
        trap - EXIT
    fi
    flock -u 8
fi
mkdir -p "$source"
if ! test -f "$metadata/editor-password"; then
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n' > "$metadata/editor-password"
fi
printf '%s\n' "$port" > "$metadata/editor-port"
printf 'bind-addr: 127.0.0.1:%s\nauth: password\npassword: %s\n' "$port" "$(cat "$metadata/editor-password")" > "$metadata/editor-config.yaml"
export PATH="$metadata/bin:$PATH"
export XDG_CONFIG_HOME="$metadata/config"
export CONTEXT="$context"
export MW_REMOTE_BRIDGE="$metadata/session.json"
export MW_REMOTE_WORKSPACE="$workspace"
export MW_REMOTE_METADATA="$metadata"
export MW_REMOTE_SOURCE="$source"
nohup "$cache/bin/code-server" --config "$metadata/editor-config.yaml" \
    --disable-telemetry --disable-update-check --user-data-dir "$metadata/editor-data" "$source" \
    < /dev/null > "$metadata/editor.log" 2>&1 6>&- 8>&- &
printf '%s\n' "$!" > "$pidfile"
for ((i=0; i<300; i++)); do
    if curl --silent --fail "http://127.0.0.1:$port/healthz" >/dev/null; then
        echo 'Browser editor is ready (authenticated SSH access).'
        exit 0
    fi
    sleep 0.1
done
tail -n 30 "$metadata/editor.log" >&2
echo 'Browser editor did not become ready.' >&2
exit 1
