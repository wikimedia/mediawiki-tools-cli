#!/usr/bin/env python3
"""Finish GitLab's blocking gate using authenticated external test results."""

import json
import os
import subprocess
import sys

from github_status import (
    GitLab, PROJECT, REQUEST_JOB, GATE_JOB, commit_sha, integration_repository, pipeline_jobs, positive_id, verify_results,
)


def main():
    if positive_id(os.environ["CI_PROJECT_ID"]) != PROJECT:
        raise ValueError("native integration is configured only for the upstream project")
    api = GitLab()  # Public metadata only; no long-lived secret in an MR job.
    pipeline_id = positive_id(os.environ["CI_PIPELINE_ID"])
    pipeline = api.get(f"/projects/{PROJECT}/pipelines/{pipeline_id}")
    sha = commit_sha(os.environ["CI_COMMIT_SHA"])
    if pipeline["sha"] != sha or pipeline["status"] in {"canceled", "canceling"}:
        raise ValueError("pipeline is canceled or its source SHA does not match")
    jobs = pipeline_jobs(api, pipeline_id)
    gate = jobs.get(GATE_JOB, {})
    bot_id = positive_id(os.environ.get("GITHUB_STATUS_BOT_ID", ""))
    # The public pipeline-jobs endpoint exposes the real job issuer. Never trust
    # GITLAB_USER_ID or a self-declared bot ID in the supplied receipt.
    if gate.get("id") != positive_id(os.environ["CI_JOB_ID"]) or gate.get("user", {}).get("id") != bot_id:
        raise ValueError("the gate must be started by the configured GitHub CI bot")
    prefix = "GITHUB_INTEGRATION_RESULTS_"
    receipts = [(positive_id(key[len(prefix):]), value) for key, value in os.environ.items()
                if key.startswith(prefix)]
    if not receipts:
        raise ValueError("no authenticated GitHub result receipt was supplied")
    receipt_id, value = max(receipts)
    receipt = json.loads(value)
    if receipt_id != max(record["id"] for record in receipt["records"]):
        raise ValueError("receipt identity does not match its status records")
    if receipt["pipeline_id"] != pipeline_id or receipt["source_sha"] != sha:
        raise ValueError("the result receipt belongs to another pipeline or source commit")
    request = jobs.get(REQUEST_JOB)
    if request is None or request["status"] != "success":
        raise ValueError("the latest native integration request has not succeeded")
    tested_sha = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    batch = verify_results(
        receipt["records"],
        bot_id=bot_id,
        request_id=request["id"], tested_sha=tested_sha, repository=integration_repository(),
    )
    print(f"All four native GitHub integration tests passed: run {batch[1]}, attempt {batch[2]}")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"Native GitHub integration gate failed: {error}", file=sys.stderr)
        sys.exit(1)
