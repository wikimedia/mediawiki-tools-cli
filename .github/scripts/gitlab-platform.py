#!/usr/bin/env python3
"""Prepare source and report one platform test; never execute MR code here."""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import urllib.request

spec = importlib.util.spec_from_file_location("native", Path(__file__).with_name("gitlab-native.py"))
native = importlib.util.module_from_spec(spec)
spec.loader.exec_module(native)


def identity():
    platform = os.environ["INTEGRATION_PLATFORM"]
    test = os.environ["TEST_NAME"]
    if platform not in {"linux-arm64", "macos-arm64", "windows-amd64"}:
        raise ValueError("unsupported integration platform")
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,63}", test):
        raise ValueError("expected a test name, without a path or arguments")
    request = f"integration-github-{platform}-{test}-request"
    if os.environ["REQUEST_JOB_NAME"] != request:
        raise ValueError("request job name does not match the platform and test")
    return platform, test, request, f"github/{platform}/{test}"


def load_request(api, job_id, *, wait=False):
    return native.load_request(api, job_id, wait=wait, request_name=identity()[2], require_gate=False)


def report(api, plan, state, url=None):
    run_id = native.positive_id(os.environ["GITHUB_RUN_ID"])
    attempt = native.positive_id(os.environ["GITHUB_RUN_ATTEMPT"])
    api.post(f"/projects/{native.PROJECT}/statuses/{plan['source_sha']}", {
        "pipeline_id": plan["pipeline_id"], "name": identity()[3], "state": state,
        "target_url": url or f"https://github.com/{native.integration_repository()}/actions/runs/{run_id}",
        "description": native.description(plan["request_id"], run_id, attempt, plan["tested_sha"]),
    })


def prepare(api, job_id, directory):
    loaded = load_request(api, job_id, wait=True)
    if loaded is None:
        native.output("active", "false")
        return
    plan, jobs = loaded
    checks = [job for name, job in jobs.items() if name == "checks" or name.startswith("checks: [")]
    if (not checks or any(job.get("status") != "success" for job in checks)
            or any(jobs.get(name, {}).get("status") != "success" for name in ("test", "build"))):
        raise ValueError("request prerequisites have not passed")
    directory.mkdir(parents=True, exist_ok=True)
    native.prepare_source(api, plan, jobs, directory)
    plan["platform"], plan["test"] = identity()[:2]
    (directory / "plan.json").write_text(json.dumps(plan))
    native.output("artifact", f"gitlab-{plan['platform']}-{job_id}-{os.environ['GITHUB_RUN_ID']}-{os.environ['GITHUB_RUN_ATTEMPT']}")
    native.output("tested_sha", plan["tested_sha"])
    native.output("active", "true")
    report(api, plan, "pending")


def test_result():
    """Recover successful test jobs when only the reporting job is rerun."""
    run_id = native.positive_id(os.environ["GITHUB_RUN_ID"])
    for attempt in range(native.positive_id(os.environ["GITHUB_RUN_ATTEMPT"]), 0, -1):
        page = 1
        while True:
            request = urllib.request.Request(
                f"https://api.github.com/repos/{native.integration_repository()}/actions/runs/{run_id}"
                f"/attempts/{attempt}/jobs?per_page=100&page={page}", headers={
                    "Authorization": "Bearer " + os.environ["GH_TOKEN"],
                    "Accept": "application/vnd.github+json", "User-Agent": "mwcli-native-ci",
                })
            with urllib.request.urlopen(request, timeout=60) as response:
                jobs = json.load(response)["jobs"]
            for job in jobs:
                if job["name"] == "Test":
                    return ("success" if job["conclusion"] == "success" else "failed"), job["html_url"]
            if len(jobs) < 100:
                break
            page += 1
    return "failed", None


def finish(api, job_id, tested_sha, prepared):
    loaded = load_request(api, job_id)
    if loaded is None:
        print("Request is canceled or superseded; no callback needed.")
        return
    plan, _ = loaded
    plan["tested_sha"] = native.commit_sha(tested_sha or plan["source_sha"])
    state, url = test_result() if prepared and tested_sha else ("failed", None)
    # Platform results are independent of the default Ubuntu integration gate.
    report(api, plan, state, url)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["prepare", "finish"])
    args = parser.parse_args()
    if os.environ["GITHUB_REPOSITORY"] != native.integration_repository():
        raise ValueError("controller must run in the configured GitHub integration repository")
    identity()
    api = native.GitLab(os.environ["GITLAB_STATUS_TOKEN"])
    job_id = native.positive_id(os.environ["REQUEST_JOB_ID"])
    if args.command == "prepare":
        prepare(api, job_id, Path("inputs"))
    else:
        finish(api, job_id, os.environ.get("TESTED_SHA", ""),
               os.environ.get("PREPARATION_RESULT") == "success")


if __name__ == "__main__":
    main()
