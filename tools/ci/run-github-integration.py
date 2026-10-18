#!/usr/bin/env python3
"""Prepare and run a selected integration test from the tested source."""

import json
import os
import platform
import re
from pathlib import Path
import shutil
import signal
import subprocess
import sys

from github_status import GitLab, PROJECT, REQUEST_JOB, pipeline_jobs
from integration_tests import TESTS


def main():
    name = sys.argv[1]
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,63}", name):
        raise ValueError("invalid integration test name")
    script = TESTS.get(name, f"test-{name}.sh")
    source = Path.cwd()
    inputs = source.parent / "inputs"
    plan = json.loads((inputs / "plan.json").read_text())
    actual = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    if actual != plan["tested_sha"]:
        raise ValueError("source snapshot does not match the prepared commit")
    (source / "bin").mkdir(exist_ok=True)
    env = os.environ.copy()
    env["MWCLI_CONTEXT_TEST"] = "1"
    if sys.platform == "linux" and platform.machine().lower() in {"amd64", "x86_64"}:
        shutil.copyfile(inputs / "mw", source / "bin/mw")
        (source / "bin/mw").chmod(0o755)
        subprocess.run(["tar", "-xzf", str(inputs / "mediawiki.tar.gz")], cwd=source, check=True)
        env["MWCLI_CI_MEDIAWIKI_SNAPSHOT"] = "1"
        compose = source / "bin/docker-compose"
        compose.write_text('#!/bin/sh\nexec docker compose "$@"\n')
        compose.chmod(0o755)
        env["PATH"] = str(source / "bin") + os.pathsep + env["PATH"]
    else:
        binary = "bin/mw.exe" if os.name == "nt" else "bin/mw"
        subprocess.run(["go", "build", "-o", binary, "."], cwd=source, env=env, check=True)
    process = subprocess.Popen(["bash", f"tests/{script}"], cwd=source, env=env,
                               start_new_session=os.name != "nt")
    api = GitLab()  # This job has no GitLab token or GitHub write token.
    try:
        while process.poll() is None:
            try:
                process.wait(timeout=30)
            except subprocess.TimeoutExpired:
                pipeline = api.get(f"/projects/{PROJECT}/pipelines/{plan['pipeline_id']}")
                request = pipeline_jobs(api, plan["pipeline_id"]).get(REQUEST_JOB, {})
                if pipeline["status"] in {"canceled", "canceling"} or request.get("id") != plan["request_id"]:
                    raise RuntimeError("GitLab pipeline canceled or request superseded")
        return process.returncode
    finally:
        if process.poll() is None:
            if os.name == "nt":
                subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"], check=False)
                process.wait()
            else:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()


if __name__ == "__main__":
    sys.exit(main())
