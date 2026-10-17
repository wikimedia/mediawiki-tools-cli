#!/usr/bin/env python3
"""Find pending jobs that the dind-tagged mwcli runner can accept."""

import json
import os
import re
import sys
import urllib.error
import urllib.request

API = "https://gitlab.wikimedia.org/api/v4/projects/16/jobs"


def eligible(job):
    return job.get("status") == "pending" and set(job.get("tag_list", [])) == {"dind"}


def find_pending(get_json, job_id=""):
    if job_id:
        if not re.fullmatch(r"[1-9][0-9]*", job_id):
            raise ValueError("job_id must be a positive integer")
        job = get_json(f"{API}/{job_id}")
        return str(job["id"]) if eligible(job) else ""

    page = 1
    while True:
        jobs = get_json(f"{API}?scope[]=pending&per_page=100&page={page}")
        for job in jobs:
            if eligible(job):
                return str(job["id"])
        if len(jobs) < 100:
            return ""
        page += 1


def main():
    token = os.environ.get("GITLAB_API_READ_TOKEN", "")
    if not token:
        raise ValueError("Add the GITLAB_API_READ_TOKEN Actions secret first")

    def get_json(url):
        request = urllib.request.Request(url, headers={"PRIVATE-TOKEN": token})
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.load(response)

    job_id = find_pending(get_json, os.environ.get("JOB_ID", ""))
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
        output.write(f"pending={'true' if job_id else 'false'}\n")
        output.write(f"job_id={job_id}\n")
    print(f"Pending dind job: {job_id}" if job_id else "No matching pending job; no worker needed.")


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as error:
        sys.exit(f"GitLab queue check failed: HTTP {error.code}")
    except (ValueError, urllib.error.URLError) as error:
        sys.exit(str(error))
