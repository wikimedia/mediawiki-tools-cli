"""Regression checks for accepting external results and protecting API credentials."""

import io
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
import urllib.error
import urllib.request
from unittest.mock import patch

from github_status import (
    API, SUMMARY_CONTEXT, GitLab, SafeRedirect, description, integration_repository, latest_jobs, verify_summary,
)

SHA = "a" * 40


def summary():
    return {
        "id": 1, "name": SUMMARY_CONTEXT, "status": "success", "author": {"id": 7},
        "description": description(19, 123, 1, SHA),
        "target_url": "https://github.com/wikimedia/mediawiki-tools-cli/actions/runs/123",
    }


class StatusTests(unittest.TestCase):
    def verify(self, value):
        return verify_summary(value, bot_id=7, request_id=19, tested_sha=SHA)

    def test_authenticates_summary_identity_independently_of_test_count(self):
        self.assertEqual(self.verify(summary()), ("19", "123", "1", SHA))
        for status in ("failed", "pending", "canceled"):
            self.assertEqual(self.verify({**summary(), "status": status}), ("19", "123", "1", SHA))

    def test_wrong_bot_request_snapshot_or_run_url_is_rejected(self):
        changes = [
            {"name": "github/individual-test"}, {"allow_failure": True},
            {"author": {"id": 99}}, {"description": "missing identity"},
            {"description": description(20, 123, 1, SHA)},
            {"description": description(19, 123, 1, "b" * 40)},
            {"description": description(19, 456, 1, SHA)},
            {"target_url": "https://github.com/other/repo/actions/runs/123"},
            {"target_url": "https://github.com/wikimedia/mediawiki-tools-cli/actions/runs/1234"},
        ]
        for change in changes:
            with self.subTest(change=change), self.assertRaises(ValueError):
                self.verify({**summary(), **change})

    def test_testing_repository_requires_explicit_selection(self):
        value = summary()
        value["target_url"] = value["target_url"].replace("wikimedia/mediawiki-tools-cli", "addshore/testing")
        with self.assertRaises(ValueError):
            self.verify(value)
        with patch.dict(os.environ, {"GITHUB_INTEGRATION_REPOSITORY": "addshore/testing"}):
            repository = integration_repository()
            self.assertEqual(repository, "addshore/testing")
            verify_summary(value, bot_id=7, request_id=19, tested_sha=SHA, repository=repository)
        with patch.dict(os.environ, {"GITHUB_INTEGRATION_REPOSITORY": "other/repository"}):
            with self.assertRaises(ValueError):
                integration_repository()

    def test_latest_failure_is_selected_over_old_success(self):
        failure = {**summary(), "id": 99, "status": "failed"}
        self.assertEqual(latest_jobs([failure, summary()])[SUMMARY_CONTEXT], failure)

    def test_retries_transient_status_api_conflicts(self):
        conflict = urllib.error.HTTPError(API, 409, "Conflict", {}, io.BytesIO())
        success = io.BytesIO(b'{"status":"success"}')
        with patch("github_status.urllib.request.build_opener") as opener, patch("github_status.time.sleep"):
            opener.return_value.open.side_effect = [conflict, success]
            self.assertEqual(GitLab("test-token").post("/projects/16/statuses/" + SHA,
                                                      {"state": "success"}), {"status": "success"})
            self.assertEqual(opener.return_value.open.call_count, 2)

    def test_artifact_redirect_does_not_send_gitlab_token_to_storage_host(self):
        request = urllib.request.Request(API + "/projects/16/jobs/1/artifacts/bin/mw",
                                         headers={"PRIVATE-TOKEN": "test-token"})
        redirected = SafeRedirect().redirect_request(request, None, 302, "Found", {},
                                                     "https://storage.example/artifact")
        self.assertIsNone(redirected.get_header("Private-token"))
        same_host = SafeRedirect().redirect_request(request, None, 302, "Found", {},
                                                    API + "/other")
        self.assertEqual(same_host.get_header("Private-token"), "test-token")
        with self.assertRaises(ValueError):
            SafeRedirect().redirect_request(request, None, 302, "Found", {}, "http://storage.example/file")


class NativeExecutionTests(unittest.TestCase):
    def test_prepares_snapshot_binary_and_preserves_test_exit_status(self):
        runner = Path(__file__).with_name("run-github-integration.py").resolve()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo = root / "repository"
            repo.mkdir()
            def git(*args):
                return subprocess.check_output(["git", *args], cwd=repo, text=True,
                                               stderr=subprocess.DEVNULL).strip()
            git("init")
            git("config", "user.name", "CI test")
            git("config", "user.email", "ci@example.invalid")
            (repo / "tests").mkdir()
            (repo / "tests/test-docker-general.sh").write_text(
                'test "$MWCLI_CI_MEDIAWIKI_SNAPSHOT" = 1\n'
                'test -z "${GITLAB_STATUS_TOKEN:-}"\n'
                'test -f .mediawiki/fixture\n./bin/mw\n'
            )
            git("add", ".")
            git("commit", "-m", "Fixture")
            sha = git("rev-parse", "HEAD")
            media = root / "mediawiki"
            media.mkdir()
            (media / "fixture").write_text("clean snapshot")
            for code in (0, 23):
                with self.subTest(exit_code=code):
                    work = root / str(code)
                    inputs = work / "inputs"
                    inputs.mkdir(parents=True)
                    git("bundle", "create", str(inputs / "source.bundle"), "HEAD")
                    (inputs / "plan.json").write_text(json.dumps({
                        "tested_sha": sha, "pipeline_id": 23, "request_id": 19,
                    }))
                    (inputs / "mw").write_text(f"#!/bin/sh\nexit {code}\n")
                    with tarfile.open(inputs / "mediawiki.tar.gz", "w:gz") as archive:
                        archive.add(media, arcname=".mediawiki")
                    source = work / "source"
                    subprocess.run(["git", "clone", str(inputs / "source.bundle"), str(source)],
                                   check=True, capture_output=True)
                    env = {key: value for key, value in os.environ.items() if key != "GITLAB_STATUS_TOKEN"}
                    result = subprocess.run(["python3", str(runner), "docker-general"],
                                            cwd=source, env=env, capture_output=True, text=True, timeout=15)
                    self.assertEqual(result.returncode, code, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
