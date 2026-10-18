#!/usr/bin/env python3
"""Build the tested source natively and smoke-test config and embedded files."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


def run(binary, source, env, *args):
    command = [str(binary), "--no-interaction", *args]
    print("+", " ".join(command), flush=True)
    result = subprocess.run(command, cwd=source, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    print(result.stdout, end="", flush=True)
    if result.returncode:
        raise SystemExit(result.returncode)
    return result.stdout


def main():
    source = Path(os.environ["RUNNER_TEMP"]) / "mwcli-source"
    binary = Path(os.environ["RUNNER_TEMP"]) / ("mw.exe" if sys.platform == "win32" else "mw")
    plan = json.loads(Path("inputs/plan.json").read_text())
    actual_sha = subprocess.check_output(
        ["git", "-C", str(source), "rev-parse", "HEAD"], text=True
    ).strip()
    if actual_sha != plan["tested_sha"]:
        raise SystemExit("source bundle does not match the prepared GitLab commit")

    with tempfile.TemporaryDirectory(prefix="mwcli-platform-smoke-") as tmp:
        home = Path(tmp) / "home"
        config = Path(tmp) / "config"
        home.mkdir()
        config.mkdir()
        env = os.environ.copy()
        env.update({
            "HOME": str(home),
            "XDG_CONFIG_HOME": str(config),
            "APPDATA": str(config),
            "USERPROFILE": str(home),
            "MWCLI_MEDIAWIKI_ENV_COMMAND": "1",
            "PORT": "8080",
            "NETWORK_SUBNET_PREFIX": "10.0.0",
        })

        run(binary, source, env, "version")
        run(binary, source, env, "config", "set", "telemetry", "platform-smoke")
        run(binary, source, env, "config", "get", "telemetry")
        run(binary, source, env, "config", "where")
        run(binary, source, env, "docker", "--context", "ci-platform-smoke", "env", "set",
            "MEDIAWIKI_VOLUMES_CODE", str(source / "mount/dev"))
        run(binary, source, env, "docker", "--context", "ci-platform-smoke", "mediawiki",
            "get-code", "--core", "--dry-run", "--gerrit-interaction-type", "http")
        run(binary, source, env, "docker", "--context", "ci-platform-smoke", "compose", "config")
        output = run(binary, source, env, "docker", "--context", "ci-platform-smoke", "where")
        if not output.strip():
            raise SystemExit("mw docker where returned no development directory")


if __name__ == "__main__":
    main()
