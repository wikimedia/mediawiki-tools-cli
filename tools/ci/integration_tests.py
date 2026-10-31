"""Shared inputs and script aliases for the integration tests."""

import hashlib
import re

TESTS = {
    "docker-general": "test-docker-general.sh",
    "all-databases": "test-docker-mw-all-dbs.sh",
    "mysql-cycle": "test-docker-mw-mysql-cycle.sh",
    "repoclient": "test-docker-recipe-repoclient.sh",
}


def download_snapshot(api, directory):
    path, checksum = (directory / "mediawiki-snapshot.txt").read_text().split()
    pattern = (r"/projects/16/packages/generic/mediawiki-ci-snapshot/"
               r"[0-9]{4}-[0-9]{2}-[0-9a-f]{64}/[0-9a-f]{64}\.tar\.gz")
    if not re.fullmatch(pattern, path) or not path.endswith(f"/{checksum}.tar.gz"):
        raise ValueError("invalid MediaWiki snapshot reference")
    archive = directory / "mediawiki.tar.gz"
    api.request(path, destination=archive)
    with archive.open("rb") as file:
        if hashlib.file_digest(file, "sha256").hexdigest() != checksum:
            raise ValueError("MediaWiki snapshot checksum mismatch")
