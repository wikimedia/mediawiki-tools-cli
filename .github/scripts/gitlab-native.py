#!/usr/bin/env python3
"""Trusted controller for native GitHub tests; never execute MR code here."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tools/ci"))
from github_status import (  # noqa: E402
    GitLab, PROJECT, REPOSITORY, REQUEST_JOB, GATE_JOB, TESTS, CONTEXTS,
    commit_sha, description, latest_jobs, parse_description, pipeline_jobs, positive_id, statuses,
)


def output(name, value):
    with open(os.environ["GITHUB_OUTPUT"], "a") as file:
        file.write(f"{name}={value}\n")


def load_request(api, job_id):
    job_id = positive_id(job_id)
    job = api.get(f"/projects/{PROJECT}/jobs/{job_id}")
    if job["name"] != REQUEST_JOB or job["status"] != "success":
        return None
    pipeline_id = positive_id(job["pipeline"]["id"])
    pipeline = api.get(f"/projects/{PROJECT}/pipelines/{pipeline_id}")
    if pipeline["status"] in {"canceled", "canceling"}:
        return None
    sha = commit_sha(pipeline["sha"])
    if job["commit"]["id"] != sha:
        raise ValueError("request and pipeline source SHA differ")
    jobs = pipeline_jobs(api, pipeline_id)
    if jobs.get(REQUEST_JOB, {}).get("id") != job_id:
        return None  # A newer request superseded this webhook/run.
    if GATE_JOB not in jobs:
        raise ValueError("pipeline has no native integration gate")
    return {"request_id": job_id, "pipeline_id": pipeline_id, "source_sha": sha}, jobs


def git(*args, cwd):
    return subprocess.check_output(["git", *args], cwd=cwd, text=True).strip()


def prepare(api, job_id, directory):
    loaded = load_request(api, job_id)
    if loaded is None:
        output("active", "false")
        return
    plan, jobs = loaded
    if jobs[GATE_JOB]["status"] == "success":
        output("active", "false")
        return
    # Serialize dispatches by request ID in workflow concurrency, then ignore
    # duplicate webhook deliveries. Explicit GitHub reruns remain possible.
    previous = {record["name"]: record for record in sorted(
        statuses(api, plan["source_sha"], plan["pipeline_id"]), key=lambda record: record["id"]
    ) if record["name"] in CONTEXTS}
    if os.environ["GITHUB_RUN_ATTEMPT"] == "1" and all(
        name in previous and previous[name]["status"] in {"success", "failed"}
        and parse_description(previous[name].get("description"))[0] == str(plan["request_id"])
        for name in CONTEXTS
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
    # Upload only four explicit files, never the controller's .git directory.
    (directory / "plan.json").write_text(json.dumps(plan))
    output("artifact", f"gitlab-{plan['request_id']}-{os.environ['GITHUB_RUN_ID']}-{os.environ['GITHUB_RUN_ATTEMPT']}")
    output("tested_sha", plan["tested_sha"])
    output("active", "true")
    report(api, plan, {name: {"state": "pending"} for name in TESTS},
           os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])


def github_results(run_id, attempt):
    """Latest conclusion per matrix job, including successful jobs from a prior attempt."""
    results = {}
    for number in range(positive_id(attempt), 0, -1):
        page = 1
        while True:
            url = (f"https://api.github.com/repos/{REPOSITORY}/actions/runs/{positive_id(run_id)}"
                   f"/attempts/{number}/jobs?per_page=100&page={page}")
            request = urllib.request.Request(url, headers={
                "Authorization": f"Bearer {os.environ['GH_TOKEN']}",
                "Accept": "application/vnd.github+json", "User-Agent": "mwcli-native-ci",
            })
            with urllib.request.urlopen(request, timeout=60) as response:
                jobs = json.load(response)["jobs"]
            for job in jobs:
                for name in TESTS:
                    if job["name"] == f"Native integration ({name}) / Test" and name not in results:
                        results[name] = {
                            "state": "success" if job["conclusion"] == "success" else "failed",
                            "url": job["html_url"],
                        }
            if len(jobs) < 100:
                break
            page += 1
    return results


def report(api, plan, results, run_id, attempt):
    identity = description(plan["request_id"], run_id, attempt, plan["tested_sha"])
    run_url = f"https://github.com/{REPOSITORY}/actions/runs/{positive_id(run_id)}"
    for name, result in results.items():
        if name not in TESTS:
            raise ValueError("unexpected integration test")
        api.post(f"/projects/{PROJECT}/statuses/{plan['source_sha']}", {
            "pipeline_id": plan["pipeline_id"], "name": f"github/{name}",
            "state": result["state"], "target_url": result.get("url", run_url), "description": identity,
        })


def finish(api, job_id, tested_sha, run_id, attempt, prepared=True):
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
    for name in TESTS:
        results.setdefault(name, {"state": "failed"})
    report(api, plan, results, run_id, attempt)
    # Re-check cancellation after posting; never revive a canceled pipeline.
    loaded = load_request(api, job_id)
    if loaded is None:
        return
    gate = loaded[1][GATE_JOB]
    if gate["status"] in {"manual", "failed"}:
        # Listing commit statuses requires authentication on GitLab 19.2.
        # Pass a nonsecret receipt when starting the gate as our bot. The gate
        # authenticates that issuer using public pipeline job metadata.
        latest = latest_jobs(statuses(api, plan["source_sha"], plan["pipeline_id"]))
        receipt = {
            "pipeline_id": plan["pipeline_id"], "source_sha": plan["source_sha"],
            "records": [latest[name] for name in CONTEXTS],
        }
        if gate["status"] == "failed" and gate.get("user", {}).get("id") != receipt["records"][0]["author"]["id"]:
            raise ValueError("gate was manually started by another user; start a new GitLab pipeline")
        # Manual variables persist on retry. Give each receipt a new key to avoid
        # GitLab's unique-key constraint, and let the gate select the newest one.
        key = f"GITHUB_INTEGRATION_RESULTS_{max(record['id'] for record in receipt['records'])}"
        # GitLab 19.2 allows playing retryable manual jobs too. Unlike /retry,
        # /play accepts the refreshed variables for this GitHub attempt.
        api.post(f"/projects/{PROJECT}/jobs/{gate['id']}/play", {
            "job_variables_attributes": [{
                "key": key, "value": json.dumps(receipt),
            }],
        })


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["prepare", "report-test", "finish"])
    args = parser.parse_args()
    if os.environ["GITHUB_REPOSITORY"] != REPOSITORY:
        raise ValueError("controller must run in the official GitHub repository")
    api = GitLab(os.environ["GITLAB_STATUS_TOKEN"])
    job_id = positive_id(os.environ["REQUEST_JOB_ID"])
    if args.command == "prepare":
        prepare(api, job_id, Path("inputs"))
    elif args.command == "report-test":
        loaded = load_request(api, job_id)
        if loaded is not None:
            plan = loaded[0]
            plan["tested_sha"] = commit_sha(os.environ["TESTED_SHA"])
            name = os.environ["TEST_NAME"]
            if name not in TESTS:
                raise ValueError("unexpected integration test")
            results = github_results(os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])
            report(api, plan, {name: results.get(name, {"state": "failed"})},
                   os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"])
    else:
        finish(api, job_id, os.environ.get("TESTED_SHA", ""),
               os.environ["GITHUB_RUN_ID"], os.environ["GITHUB_RUN_ATTEMPT"],
               prepared=os.environ.get("PREPARATION_RESULT") == "success")


if __name__ == "__main__":
    main()
