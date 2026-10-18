#!/usr/bin/env python3
"""Trusted controller for native GitHub tests; never execute MR code here."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tools/ci"))
from github_status import (  # noqa: E402
    GitLab, PROJECT, REPOSITORY, REQUEST_JOB, GATE_JOB, SUMMARY_CONTEXT,
    commit_sha, description, integration_repository, parse_description, pipeline_jobs, positive_id, statuses,
)

from integration_tests import TESTS  # noqa: E402


def output(name, value):
    with open(os.environ["GITHUB_OUTPUT"], "a") as file:
        file.write(f"{name}={value}\n")


def load_request(api, job_id, *, wait=False, request_name=REQUEST_JOB, require_gate=True):
    job_id = positive_id(job_id)
    job = api.get(f"/projects/{PROJECT}/jobs/{job_id}")
    # Dispatch happens inside the request job; wait for GitLab to confirm its exit.
    deadline = time.monotonic() + 6 * 60
    while wait and job["name"] == request_name and job["status"] == "running":
        if time.monotonic() >= deadline:
            raise ValueError("integration request did not finish")
        time.sleep(3)
        job = api.get(f"/projects/{PROJECT}/jobs/{job_id}")
    if job["name"] != request_name or job["status"] != "success":
        return None
    pipeline_id = positive_id(job["pipeline"]["id"])
    pipeline = api.get(f"/projects/{PROJECT}/pipelines/{pipeline_id}")
    if pipeline["status"] in {"canceled", "canceling"}:
        return None
    sha = commit_sha(pipeline["sha"])
    if job["commit"]["id"] != sha:
        raise ValueError("request and pipeline source SHA differ")
    jobs = pipeline_jobs(api, pipeline_id)
    if jobs.get(request_name, {}).get("id") != job_id:
        return None  # A newer request superseded this run.
    if require_gate and GATE_JOB not in jobs:
        raise ValueError("pipeline has no native integration gate")
    return {"request_id": job_id, "pipeline_id": pipeline_id, "source_sha": sha}, jobs


def git(*args, cwd):
    return subprocess.check_output(["git", *args], cwd=cwd, text=True).strip()


def prepare(api, job_id, directory):
    loaded = load_request(api, job_id, wait=True)
    if loaded is None:
        output("active", "false")
        return
    plan, jobs = loaded
    if jobs[GATE_JOB]["status"] == "success":
        output("active", "false")
        return
    # Serialize dispatches by request ID in workflow concurrency, then ignore
    # duplicate dispatches. Explicit GitHub reruns remain possible.
    previous = {record["name"]: record for record in sorted(
        statuses(api, plan["source_sha"], plan["pipeline_id"]), key=lambda record: record["id"]
    ) if record["name"] == SUMMARY_CONTEXT}
    if os.environ["GITHUB_RUN_ATTEMPT"] == "1" and all(
        name in previous and previous[name]["status"] in {"success", "failed"}
        and parse_description(previous[name].get("description"))[0] == str(plan["request_id"])
        for name in (SUMMARY_CONTEXT,)
    ):
        output("active", "false")
        return
    for name in ("build", "prepare-mediawiki"):
        if jobs.get(name, {}).get("status") != "success":
            raise ValueError(f"{name} has not succeeded")
    directory.mkdir(parents=True, exist_ok=True)
    for job_name, artifact, filename in (
        ("build", "bin/mw", "mw"),
        ("prepare-mediawiki", ".cache/mediawiki.tar.gz", "mediawiki.tar.gz"),
    ):
        api.request(f"/projects/{PROJECT}/jobs/{jobs[job_name]['id']}/artifacts/{artifact}",
                    destination=directory / filename)
    prepare_source(api, plan, jobs, directory)
    # Upload only four explicit files, never the controller's .git directory.
    (directory / "plan.json").write_text(json.dumps(plan))
    output("artifact", f"gitlab-{plan['request_id']}-{os.environ['GITHUB_RUN_ID']}-{os.environ['GITHUB_RUN_ATTEMPT']}")
    output("tested_sha", plan["tested_sha"])
    output("active", "true")
    post_status(api, plan, SUMMARY_CONTEXT, {"state": "pending"},
                os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])


def prepare_source(api, plan, jobs, directory):
    """Prepare the exact branch or merged MR source without executing it."""
    source = directory / "repository.git"
    source.mkdir()
    git("init", "--bare", cwd=source)
    if "prepare-merge" in jobs:
        merge = jobs["prepare-merge"]
        if merge["status"] != "success":
            raise ValueError("prepare-merge has not succeeded")
        api.request(f"/projects/{PROJECT}/jobs/{merge['id']}/artifacts/.ci-merge.bundle",
                    destination=directory / "source.bundle")
        git("fetch", str((directory / "source.bundle").resolve()), "HEAD", cwd=source)
    else:
        git("fetch", "https://gitlab.wikimedia.org/repos/releng/cli.git", plan["source_sha"], cwd=source)
    plan["tested_sha"] = commit_sha(git("rev-parse", "FETCH_HEAD", cwd=source))
    git("merge-base", "--is-ancestor", plan["source_sha"], plan["tested_sha"], cwd=source)
    if "prepare-merge" not in jobs:
        if plan["tested_sha"] != plan["source_sha"]:
            raise ValueError("unexpected branch checkout")
        git("update-ref", "refs/heads/snapshot", plan["tested_sha"], cwd=source)
        git("symbolic-ref", "HEAD", "refs/heads/snapshot", cwd=source)
        git("bundle", "create", str((directory / "source.bundle").resolve()), "HEAD", cwd=source)


def github_results(run_id, attempt, job_names=None):
    """Latest conclusion per matrix job, including successful jobs from a prior attempt."""
    results = {}
    for number in range(positive_id(attempt), 0, -1):
        page = 1
        while True:
            url = (f"https://api.github.com/repos/{integration_repository()}/actions/runs/{positive_id(run_id)}"
                   f"/attempts/{number}/jobs?per_page=100&page={page}")
            request = urllib.request.Request(url, headers={
                "Authorization": f"Bearer {os.environ['GH_TOKEN']}",
                "Accept": "application/vnd.github+json", "User-Agent": "mwcli-native-ci",
            })
            with urllib.request.urlopen(request, timeout=60) as response:
                jobs = json.load(response)["jobs"]
            for job in jobs:
                if job_names is None:
                    match = re.fullmatch(r"Integration \(([a-z][a-z0-9/-]*)\) / Test", job["name"])
                    names = [match[1]] if match else []
                else:
                    names = [name for name, job_name in job_names.items() if job["name"] == job_name]
                for name in names:
                    if name not in results:
                        results[name] = {
                            "state": "success" if job["conclusion"] == "success" else "failed",
                            "url": job["html_url"],
                        }
            if len(jobs) < 100:
                break
            page += 1
    return results


def report(api, plan, results, run_id, attempt):
    for name, result in results.items():
        if not re.fullmatch(r"[a-z][a-z0-9-]*(?:/[a-z][a-z0-9-]*)?", name) or name == "integration":
            raise ValueError("unexpected integration test")
        post_status(api, plan, f"github/{name}", result, run_id, attempt)


def post_status(api, plan, context, result, run_id, attempt):
    run_url = f"https://github.com/{integration_repository()}/actions/runs/{positive_id(run_id)}"
    api.post(f"/projects/{PROJECT}/statuses/{plan['source_sha']}", {
        "pipeline_id": plan["pipeline_id"], "name": context,
        "state": result["state"], "target_url": result.get("url", run_url),
        "description": description(plan["request_id"], run_id, attempt, plan["tested_sha"]),
    })


def platform_identity():
    platform, test = os.environ["INTEGRATION_PLATFORM"], os.environ["TEST_NAME"]
    if platform not in {"linux-arm64", "macos-arm64", "windows-amd64"}:
        raise ValueError("unsupported integration platform")
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,63}", test):
        raise ValueError("expected a test name, without a path or arguments")
    request = f"integration-github-{platform}-{test}-request"
    if os.environ["REQUEST_JOB_NAME"] != request:
        raise ValueError("request job name does not match the platform and test")
    return request, f"github/{platform}/{test}", TESTS.get(test, f"test-{test}.sh")


def prepare_platform(api, job_id, directory):
    request, context, script = platform_identity()
    loaded = load_request(api, job_id, wait=True, request_name=request, require_gate=False)
    if loaded is None:
        output("active", "false")
        return
    plan, jobs = loaded
    checks = [job for name, job in jobs.items() if name == "checks" or name.startswith("checks: [")]
    if (not checks or any(job.get("status") != "success" for job in checks)
            or any(jobs.get(name, {}).get("status") != "success" for name in ("test", "build"))):
        raise ValueError("request prerequisites have not passed")
    directory.mkdir(parents=True, exist_ok=True)
    prepare_source(api, plan, jobs, directory)
    git("cat-file", "-e", f"{plan['tested_sha']}:tests/{script}", cwd=directory / "repository.git")
    output("artifact", f"gitlab-{job_id}-{os.environ['GITHUB_RUN_ID']}-{os.environ['GITHUB_RUN_ATTEMPT']}")
    output("tested_sha", plan["tested_sha"])
    output("test_script", script)
    output("active", "true")
    post_status(api, plan, context, {"state": "pending"}, os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])


def finish_platform(api, job_id, tested_sha, run_id, attempt, prepared=True):
    request, context, _ = platform_identity()
    loaded = load_request(api, job_id, request_name=request, require_gate=False)
    if loaded is None:
        return
    plan, _ = loaded
    plan["tested_sha"] = commit_sha(tested_sha or plan["source_sha"])
    results = github_results(run_id, attempt, {context: "Test"}) if prepared and tested_sha else {}
    # Each platform reports independently, even after the default Ubuntu gate passes.
    post_status(api, plan, context, results.get(context, {"state": "failed"}), run_id, attempt)


def finish(api, job_id, tested_sha, run_id, attempt, prepared=True, jobs_succeeded=False):
    loaded = load_request(api, job_id)
    if loaded is None:
        print("Request is canceled or superseded; no callback needed.")
        return
    plan, jobs = loaded
    if jobs[GATE_JOB]["status"] == "success":
        return
    # If preparation failed, publish failures; this SHA can never pass the gate
    # for an MR's merged checkout. No missing callback should appear successful.
    plan["tested_sha"] = commit_sha(tested_sha or plan["source_sha"])
    results = github_results(run_id, attempt) if prepared and tested_sha else {}
    report(api, plan, results, run_id, attempt)
    passed = prepared and jobs_succeeded and bool(results) and all(
        result["state"] == "success" for result in results.values())
    post_status(api, plan, SUMMARY_CONTEXT, {"state": "success" if passed else "failed"}, run_id, attempt)
    if not passed:
        return  # The normal waiting job observes the failures itself.
    loaded = load_request(api, job_id)
    if loaded is not None:
        gate = loaded[1][GATE_JOB]
        if gate["status"] == "failed":
            # A successful GitHub rerun can recover a failed waiting job.
            # No manual play or result variables are involved.
            api.post(f"/projects/{PROJECT}/jobs/{gate['id']}/retry", {})


def workflow_jobs_succeeded(value):
    """The trusted workflow supplies every dependency of its final report job."""
    needs = json.loads(value)
    return (isinstance(needs, dict) and "prepare" in needs and len(needs) >= 2
            and all(isinstance(job, dict) and job.get("result") == "success" for job in needs.values()))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["prepare", "report-test", "finish", "prepare-platform", "finish-platform"])
    args = parser.parse_args()
    if os.environ["GITHUB_REPOSITORY"] != integration_repository():
        raise ValueError("controller must run in the configured GitHub integration repository")
    api = GitLab(os.environ["GITLAB_STATUS_TOKEN"])
    job_id = positive_id(os.environ["REQUEST_JOB_ID"])
    if args.command == "prepare-platform":
        prepare_platform(api, job_id, Path("inputs"))
    elif args.command == "finish-platform":
        finish_platform(api, job_id, os.environ.get("TESTED_SHA", ""),
                        os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"],
                        prepared=os.environ.get("PREPARATION_RESULT") == "success")
    elif args.command == "prepare":
        prepare(api, job_id, Path("inputs"))
    elif args.command == "report-test":
        loaded = load_request(api, job_id)
        if loaded is not None:
            plan = loaded[0]
            plan["tested_sha"] = commit_sha(os.environ["TESTED_SHA"])
            name = os.environ["TEST_NAME"]
            results = github_results(os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])
            report(api, plan, {name: results.get(name, {"state": "failed"})},
                   os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])
    else:
        finish(api, job_id, os.environ.get("TESTED_SHA", ""),
               os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"],
               prepared=os.environ.get("PREPARATION_RESULT") == "success",
               jobs_succeeded=workflow_jobs_succeeded(os.environ.get("WORKFLOW_NEEDS", "{}")))


if __name__ == "__main__":
    main()
