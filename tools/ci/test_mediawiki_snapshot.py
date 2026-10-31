"""Checks for shared snapshot downloads and cleanup."""

from datetime import datetime, timezone
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock

from cleanup_mediawiki_snapshots import expired_snapshots
from integration_tests import download_snapshot


class DownloadTests(unittest.TestCase):
    def test_download_verifies_checksum(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            content = b"shared archive"
            checksum = hashlib.sha256(content).hexdigest()
            path = f"/projects/16/packages/generic/mediawiki-ci-snapshot/2026-44-{'a' * 64}/{checksum}.tar.gz"
            (directory / "mediawiki-snapshot.txt").write_text(f"{path} {checksum}\n")
            api = Mock()
            api.request.side_effect = lambda path, destination: destination.write_bytes(content)
            download_snapshot(api, directory)
            self.assertEqual((directory / "mediawiki.tar.gz").read_bytes(), content)
            api.request.side_effect = lambda path, destination: destination.write_bytes(b"corrupt")
            with self.assertRaisesRegex(ValueError, "checksum"):
                download_snapshot(api, directory)

    def test_manifest_rejects_foreign_projects_urls_and_bad_checksums(self):
        with tempfile.TemporaryDirectory() as temporary:
            manifest = Path(temporary) / "mediawiki-snapshot.txt"
            checksum = "a" * 64
            path = f"/projects/16/packages/generic/mediawiki-ci-snapshot/2026-44-{'b' * 64}/{checksum}.tar.gz"
            for value in [f"https://evil.example/file {checksum}",
                          f"{path.replace('/16/', '/17/')} {checksum}",
                          f"{path} bad", f"{path} {checksum} extra", "fixture"]:
                with self.subTest(value=value), self.assertRaises(ValueError):
                    manifest.write_text(value)
                    download_snapshot(Mock(), manifest.parent)


class CleanupTests(unittest.TestCase):
    def test_cleanup_preserves_recent_and_unrelated_packages(self):
        old = dict(id=1, name='mediawiki-ci-snapshot', package_type='generic',
                   version='2026-39-' + 'a' * 64, created_at='2026-09-24T00:00:00Z')
        packages = [old, {**old, 'id': 2, 'created_at': '2026-10-20T00:00:00Z'},
                    {**old, 'id': 3, 'name': 'mwcli'}, {**old, 'id': 4, 'version': 'release'},
                    {**old, 'id': 5, 'created_at': '2026-10-03T00:00:00Z'},
                    {**old, 'id': 6, 'package_type': 'npm'}]
        self.assertEqual(expired_snapshots(packages, datetime(2026, 10, 31, tzinfo=timezone.utc)), [1])
