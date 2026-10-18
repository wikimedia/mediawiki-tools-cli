#!/usr/bin/env bash
# Exercise the native mw executable against an actual Docker engine.
set -euo pipefail

run=(docker run --rm)
case "$(uname -s)" in
    MINGW*|MSYS*)
        powershell.exe -NoLogo -NoProfile -NonInteractive -Command \
            '$ErrorActionPreference = "Stop"; Start-Service docker'
        image=mcr.microsoft.com/windows/nanoserver:ltsc2025
        smoke=(cmd /c echo MWCLI_REAL_DOCKER)
        run+=(--isolation=process)
        ;;
    Darwin)
        # A foreign-architecture QEMU guest uses software emulation rather than
        # the nested hardware virtualization unavailable on hosted macOS ARM.
        if ! docker info >/dev/null 2>&1; then
            brew install colima docker docker-compose qemu lima-additional-guestagents
            colima start --vm-type qemu --arch x86_64 --cpu 2 --memory 2 --disk 10 --mount-type 9p
        fi
        image=alpine:3.22
        smoke=(echo MWCLI_REAL_DOCKER)
        ;;
    *)
        image=alpine:3.22
        smoke=(echo MWCLI_REAL_DOCKER)
        ;;
esac

echo 'Checking real Docker engine (no API fixture)'
docker version
docker info --format 'Engine: {{.OSType}}/{{.Architecture}} ({{.ServerVersion}})'

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
# Capture the existing real engine endpoint and Compose plugin before isolating
# Docker and mw configuration. Do not alter the runner's saved Docker context.
python3 - "$root" <<'PY'
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess
import sys
import shutil

root = Path(sys.argv[1]).resolve()
context = json.loads(subprocess.check_output(['docker', 'context', 'inspect'], text=True))[0]
plugins = json.loads(subprocess.check_output(
    ['docker', 'info', '--format', '{{json .ClientInfo.Plugins}}'], text=True)) or []
config = root / 'docker'
config.mkdir()
plugin_dirs = {str(Path(plugin['Path']).parent) for plugin in plugins if plugin.get('Path')}
compose = shutil.which('docker-compose')
if compose:
    plugin_dirs.add(str(Path(compose).resolve().parent))
(config / 'config.json').write_text(json.dumps({'cliPluginsExtraDirs': sorted(plugin_dirs)}))
config_home = root / 'config home'
(config_home / 'mwcli').mkdir(parents=True)
(config_home / 'mwcli/config.json').write_text(json.dumps({
    'telemetry': 'no',
    'timer_last_update_checked': datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
}))
for name, value in {
    'docker-config': str(config), 'config-home': str(config_home),
    'host': context['Endpoints']['docker']['Host'],
    'expected': str(config_home / 'mwcli/mwdd/real-engine-check'),
}.items():
    (root / name).write_text(value)
PY
export DOCKER_CONFIG="$(cat "$root/docker-config")"
export XDG_CONFIG_HOME="$(cat "$root/config-home")"
export APPDATA="$XDG_CONFIG_HOME" LOCALAPPDATA="$XDG_CONFIG_HOME"
export MWCLI_CONTEXT_TEST=1
unset GITLAB_CI CI_PROJECT_DIR DOCKER_HOST DOCKER_CONTEXT DOCKER_API_VERSION
unset DOCKER_TLS DOCKER_TLS_VERIFY DOCKER_CERT_PATH
unset MWCLI_MEDIAWIKI_ENV_COMMAND MW_DOCKER_MEDIAWIKI_GET_CODE
host="$(cat "$root/host")"
expected="$(cat "$root/expected")"
binary=./bin/mw
[[ ! -f ./bin/mw.exe ]] || binary=./bin/mw.exe

docker context create mwcli-real-engine --docker "host=$host"
docker context use mwcli-real-engine
# The actual Compose plugin must be available to mw, including on macOS.
docker compose version
# Disable MSYS path conversion for cmd /c on Windows.
output="$(MSYS_NO_PATHCONV=1 "${run[@]}" "$image" "${smoke[@]}")"
echo "$output"
[[ "$output" == *MWCLI_REAL_DOCKER* ]] || { echo 'FAIL: real Docker container did not run'; exit 1; }
echo "PASS: real Docker engine ran a container ($host)"

check_context() {
    local label="$1" output
    echo "Checking real Docker context: $label ($host)"
    if ! output="$("$binary" --no-interaction dev --context=real-engine-check where 2>&1)"; then
        echo "$output"
        exit 1
    fi
    echo "$output"
    [[ "$output" == *"$expected"* ]] || { echo 'FAIL: CLI did not finish against the real engine'; exit 1; }
    echo "PASS: native mw reached real Docker using $label"
}

check_context 'saved context'
docker context use default
export DOCKER_CONTEXT=mwcli-real-engine
check_context 'DOCKER_CONTEXT'
export DOCKER_CONTEXT=missing-context DOCKER_HOST="$host"
check_context 'DOCKER_HOST overriding missing context'
