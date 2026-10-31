#!/usr/bin/env python3
"""Refresh the exact Wikimedia CentralAuth wiki endpoint allowlist."""

import datetime
import json
from pathlib import Path
from urllib.parse import urlparse
from urllib.request import Request, urlopen

MATRIX = "https://en.wikipedia.org/w/api.php?action=sitematrix&format=json"
SUL = "https://raw.githubusercontent.com/wikimedia/operations-mediawiki-config/master/dblists/sul.dblist"


def fetch(url):
    request = Request(url, headers={"User-Agent": "a-wikimedia-cli realm-list updater"})
    with urlopen(request, timeout=30) as response:
        return response.read().decode()


def main():
    matrix = json.loads(fetch(MATRIX))["sitematrix"]
    members = {line.strip() for line in fetch(SUL).splitlines() if line.strip() and not line.startswith("#")}
    sites = list(matrix["specials"])
    for value in matrix.values():
        if isinstance(value, dict):
            sites.extend(value.get("site", []))
    endpoints = set()
    for site in sites:
        if site["dbname"] not in members or "private" in site:
            continue
        parsed = urlparse(site["url"])
        if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.query or parsed.fragment or parsed.path not in ("", "/"):
            raise ValueError("Unexpected site-matrix endpoint; review before trusting it")
        endpoints.add(site["url"].rstrip("/") + "/w/api.php")
    if len(endpoints) < 500:
        raise ValueError("Unexpectedly small realm list; existing file was not changed")
    output = Path(__file__).resolve().parents[1] / "internal/wikiauth/wikimedia.txt"
    today = datetime.datetime.now(datetime.timezone.utc).date()
    output.write_text(
        f"# Wikimedia CentralAuth API endpoints, refreshed {today}.\n"
        f"# Sources: {MATRIX}\n# {SUL}\n" + "\n".join(sorted(endpoints)) + "\n"
    )
    print(f"Wrote {len(endpoints)} trusted wiki endpoints to {output}")


if __name__ == "__main__":
    main()
