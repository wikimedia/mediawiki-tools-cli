#!/usr/bin/env python3
"""Run an allowlisted native test without callback credentials."""

import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

from github_status import GitLab, PROJECT, REQUEST_JOB, TESTS, pipeline_jobs


def main():
    name = sys.argv[1]
    script = TESTS[name]
    inputs = Path("inputs").resolve()
    plan = json.loads((inputs / "plan.json").read_text())
    source = Path("source").resolve()
    source.mkdir()
    subprocess.run(["git", "init", str(source)], check=True)
    subprocess.run(["git", "-C", str(source), "fetch", str(inputs / "source.bundle"), "HEAD"], check=True)
    subprocess.run(["git", "-C", str(source), "checkout", "--detach", "FETCH_HEAD"], check=True)
    actual = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    if actual != plan["tested_sha"]:
        raise ValueError("source snapshot does not match the prepared commit")
    (source / "bin").mkdir(exist_ok=True)
    shutil.copyfile(inputs / "mw", source / "bin/mw")
    (source / "bin/mw").chmod(0o755)
    subprocess.run(["tar", "-xzf", str(inputs / "mediawiki.tar.gz")], cwd=source, check=True)
    env = os.environ.copy()
    env["MWCLI_CI_MEDIAWIKI_SNAPSHOT"] = "1"
    env["MWCLI_CONTEXT_TEST"] = "1"
    process = subprocess.Popen(["bash", f"tests/{script}"], cwd=source, env=env, start_new_session=True)
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
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()


if __name__ == "__main__":
    sys.exit(main())
