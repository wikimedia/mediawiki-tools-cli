#!/usr/bin/env python3
"""Remove only shared MediaWiki snapshot packages older than four weeks."""

from datetime import datetime, timedelta, timezone
import json
import os
import re
import urllib.request

NAME = "mediawiki-ci-snapshot"


def expired_snapshots(packages, now):
    cutoff = now - timedelta(days=28)
    return [package["id"] for package in packages
            if package.get("name") == NAME and package.get("package_type") == "generic"
            and re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9a-f]{64}", package.get("version", ""))
            and datetime.fromisoformat(package["created_at"].replace("Z", "+00:00")) < cutoff]


def main():
    base = f"{os.environ['CI_API_V4_URL']}/projects/{int(os.environ['CI_PROJECT_ID'])}/packages"
    headers = {"JOB-TOKEN": os.environ["CI_JOB_TOKEN"], "User-Agent": "a-wikimedia-cli-ci (https://a-wikimedia-cli.toolforge.org/)"}

    def request(url, method="GET"):
        with urllib.request.urlopen(urllib.request.Request(url, headers=headers, method=method), timeout=60) as response:
            return json.load(response) if method == "GET" else None

    packages, page = [], 1
    while True:
        batch = request(f"{base}?package_type=generic&package_name={NAME}&per_page=100&page={page}")
        packages.extend(batch)
        if len(batch) < 100:
            break
        page += 1
    # Collect before deleting, so pagination cannot skip shifted records.
    for identifier in expired_snapshots(packages, datetime.now(timezone.utc)):
        print(f"Removing expired MediaWiki snapshot package {identifier}", flush=True)
        request(f"{base}/{int(identifier)}", "DELETE")


if __name__ == "__main__":
    main()
