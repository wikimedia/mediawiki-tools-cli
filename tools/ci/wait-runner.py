#!/usr/bin/env python3
"""Install a separate lightweight runner service; read its credentials from stdin.

Run as root on an existing mwcli runner. Input is the JSON response from
POST /user/runners. The existing Docker runner and Docker daemon are unchanged.
"""

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

DIRECTORY = Path("/etc/gitlab-runner-wait")
WORK = Path("/var/lib/gitlab-runner-wait")
UNIT = Path("/etc/systemd/system/gitlab-runner-wait.service")
SERVICE = "gitlab-runner-wait"


def atomic_write(path, content, mode):
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as stream:
        os.fchmod(stream.fileno(), mode)
        stream.write(content)
        temporary = stream.name
    os.replace(temporary, path)


def main():
    import tomllib  # Debian 13 supplies Python 3.13.

    if os.geteuid() != 0:
        raise ValueError("run as root on the runner VM")
    existing = Path("/etc/gitlab-runner/config.toml")
    before = hashlib.sha256(existing.read_bytes()).hexdigest()
    data = json.load(sys.stdin)
    runner_id = int(data["id"])
    config = DIRECTORY / "config.toml"
    token = data.get("token")
    if token is None and config.exists():
        token = tomllib.loads(config.read_text())["runners"][0]["token"]
    if runner_id <= 0 or not isinstance(token, str) or not re.fullmatch(r"glrt-[A-Za-z0-9_.-]+", token):
        raise ValueError("expected a runner ID and authentication token from POST /user/runners")
    DIRECTORY.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(DIRECTORY, 0o700)
    if config.exists():
        previous = tomllib.loads(config.read_text())
        if [r["id"] for r in previous.get("runners", [])] != [runner_id]:
            raise ValueError("another waiting runner already exists; do not replace it")
    cpu = max(os.sched_getaffinity(0))
    name = os.uname().nodename.split(".")[0] + "-wait"
    content = f'''concurrent = 50
check_interval = 3
shutdown_timeout = 30

[[runners]]
  id = {runner_id}
  name = {json.dumps(name)}
  url = "https://gitlab.wikimedia.org/"
  token = {json.dumps(token)}
  executor = "docker"
  limit = 50
  request_concurrency = 5
  output_limit = 1024
  environment = ["FF_NETWORK_PER_BUILD=false"]
  [runners.docker]
    image = "python:3.13-alpine"
    network_mode = "bridge"
    allowed_images = ["python:3.13-alpine"]
    allowed_services = [""]
    privileged = false
    services_privileged = false
    disable_cache = true
    volumes = ["/opt/mwcli-wait:/opt/mwcli-wait:ro", "/run/mwcli-wait:/run/mwcli-wait:ro"]
    pull_policy = "if-not-present"
    memory = "64m"
    memory_swap = "64m"
    cpus = "0.10"
    cpu_shares = 64
    cpuset_cpus = "{cpu}"
    pids_limit = 64
    cap_drop = ["ALL"]
    security_opt = ["no-new-privileges:true"]
'''
    tomllib.loads(content)
    helpers = Path("/opt/mwcli-wait")
    helpers.mkdir(mode=0o755, parents=True, exist_ok=True)
    for filename in ("github_status.py", "wait-github.py", "wait-status.py"):
        atomic_write(helpers / filename, Path(__file__).with_name(filename).read_text(), 0o644)
    Path("/run/mwcli-wait").mkdir(mode=0o755, parents=True, exist_ok=True)
    if "read_token" in data:
        atomic_write(DIRECTORY / "status-token", data["read_token"].strip() + "\n", 0o600)
    if "bot_id" in data:
        repository = data.get("repository", "wikimedia/mediawiki-tools-cli")
        if repository not in {"wikimedia/mediawiki-tools-cli", "addshore/testing"}:
            raise ValueError("unsupported integration repository")
        status_config = {"bot_id": int(data["bot_id"]), "repository": repository}
        atomic_write(Path("/etc/mwcli-wait-status.json"), json.dumps(status_config), 0o644)
    status_unit = """[Unit]
Description=MWCLI read-only integration result reader
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/bin/python3 /opt/mwcli-wait/wait-status.py
DynamicUser=yes
RuntimeDirectory=mwcli-wait
RuntimeDirectoryMode=0755
RuntimeDirectoryPreserve=yes
LoadCredential=gitlab-read-token:/etc/gitlab-runner-wait/status-token
Restart=on-failure
RestartSec=5
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
MemoryMax=128M
TasksMax=24

[Install]
WantedBy=multi-user.target
"""
    atomic_write(Path("/etc/systemd/system/mwcli-wait-status.service"), status_unit, 0o644)
    atomic_write(config, content, 0o600)
    WORK.mkdir(mode=0o750, parents=True, exist_ok=True)
    unit = f'''[Unit]
Description=MWCLI lightweight waiting runner
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
ExecStart=/usr/bin/gitlab-runner run --config {config} --working-directory {WORK} --service {SERVICE} --user gitlab-runner
Restart=always
RestartSec=5
KillSignal=SIGQUIT
TimeoutStopSec=3600

[Install]
WantedBy=multi-user.target
'''
    atomic_write(UNIT, unit, 0o644)
    result = subprocess.run(["gitlab-runner", "verify", "--config", str(config)], capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError("new runner verification failed; service was not started")
    subprocess.run(["systemctl", "daemon-reload"], check=True)
    subprocess.run(["systemctl", "enable", "--now", SERVICE], check=True, capture_output=True)
    subprocess.run(["systemctl", "is-active", "--quiet", SERVICE], check=True)
    if (DIRECTORY / "status-token").exists() and Path("/etc/mwcli-wait-status.json").exists():
        subprocess.run(["systemctl", "enable", "mwcli-wait-status"], check=True, capture_output=True)
        subprocess.run(["systemctl", "restart", "mwcli-wait-status"], check=True)
        subprocess.run(["systemctl", "is-active", "--quiet", "mwcli-wait-status"], check=True)

    if hashlib.sha256(existing.read_bytes()).hexdigest() != before:
        raise RuntimeError("existing runner configuration changed during installation")
    print(json.dumps({"name": name, "runner_id": runner_id, "slots": 50,
                      "memory_per_job": "64 MiB", "cpu_set": str(cpu),
                      "service": SERVICE, "existing_config_unchanged": True}))


if __name__ == "__main__":
    main()
