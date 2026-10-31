"""Checks for shared snapshot downloads."""

import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock

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
