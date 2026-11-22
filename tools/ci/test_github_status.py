"""Regression checks for accepting external results and protecting API credentials."""

import io
import hashlib
from contextlib import chdir
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
from types import SimpleNamespace
import unittest
import urllib.error
import urllib.request
from unittest.mock import patch

from github_status import (
    API, LEGACY_CLONE_URL, SUMMARY_CONTEXT, GitLab, SafeRedirect, description, integration_repository, latest_jobs,
    verify_summary,
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

    def test_clone_url_follows_project_id_across_a_transfer(self):
        api = GitLab()
        for url in (LEGACY_CLONE_URL, "https://gitlab.wikimedia.org/repos/a-wikimedia-cli/wm.git"):
            with self.subTest(url=url), patch.object(api, "get", return_value={
                "id": 16, "http_url_to_repo": url,
            }) as get:
                self.assertEqual(api.project_clone_url(), url)
                get.assert_called_once_with("/projects/16")

    def test_clone_url_falls_back_when_project_metadata_is_unavailable(self):
        api = GitLab()
        for error in (urllib.error.HTTPError(API, 404, "Not found", {}, None),
                      urllib.error.HTTPError(API, 503, "Unavailable", {}, None),
                      urllib.error.URLError("Timed out")):
            with self.subTest(error=error), patch.object(api, "get", side_effect=error):
                self.assertEqual(api.project_clone_url(), LEGACY_CLONE_URL)

    def test_clone_url_rejects_unexpected_project_metadata(self):
        api = GitLab()
        for project in (
            {"id": 6, "http_url_to_repo": LEGACY_CLONE_URL},
            {"id": 16, "http_url_to_repo": "https://storage.example/cli.git"},
            {"id": 16, "http_url_to_repo": "https://gitlab.wikimedia.org@storage.example/cli.git"},
        ):
            with self.subTest(project=project), patch.object(api, "get", return_value=project):
                with self.assertRaisesRegex(ValueError, "unexpected clone URL"):
                    api.project_clone_url()


class NativeExecutionTests(unittest.TestCase):
    def test_local_integration_tests_prefer_the_fresh_build(self):
        functions = Path(__file__).resolve().parents[2] / "tests/functions.sh"
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "tests").mkdir()
            (root / "bin").mkdir()
            local_functions = root / "tests/functions.sh"
            shutil.copyfile(functions, local_functions)
            wm = root / "bin/wm"
            mw = root / "bin/mw"
            for binary, timestamp in ((wm, 10), (mw, 20)):
                binary.touch()
                binary.chmod(0o755)
                os.utime(binary, (timestamp, timestamp))
            command = ["bash", "-c", 'OSTYPE=linux-gnu; source "$1"; test_cli_binary "$2"',
                       "fixture", str(local_functions)]
            self.assertEqual(Path(subprocess.check_output(command + [""], text=True).strip()).name, "mw")
            self.assertEqual(Path(subprocess.check_output(command + [str(wm)], text=True).strip()).name, "wm")
            os.utime(wm, (30, 30))
            self.assertEqual(Path(subprocess.check_output(command + [""], text=True).strip()).name, "wm")

    def test_windows_selects_git_bash_instead_of_the_wsl_launcher(self):
        spec = importlib.util.spec_from_file_location(
            "worker", Path(__file__).with_name("run-github-integration.py"))
        worker = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(worker)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "Program Files/Git"
            bash = root / "bin/bash.exe"
            bash.parent.mkdir(parents=True)
            bash.touch()
            for location in ("cmd/git.exe", "bin/git.exe", "mingw64/bin/git.exe"):
                with self.subTest(git=location), patch.object(
                    worker, "os", SimpleNamespace(name="nt")
                ), patch.object(worker.shutil, "which", return_value=str(root / location)) as which:
                    self.assertEqual(worker.test_shell(), str(bash))
                    which.assert_called_once_with("git")

    def test_windows_does_not_fall_back_to_wsl_when_git_bash_is_missing(self):
        spec = importlib.util.spec_from_file_location(
            "worker", Path(__file__).with_name("run-github-integration.py"))
        worker = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(worker)
        with patch.object(worker, "os", SimpleNamespace(name="nt")), patch.object(
            worker.shutil, "which", return_value=None
        ), self.assertRaisesRegex(RuntimeError, "Git for Windows Bash is required"):
            worker.test_shell()

    def test_prepares_snapshot_binary_and_preserves_test_exit_status(self):
        spec = importlib.util.spec_from_file_location(
            "worker", Path(__file__).with_name("run-github-integration.py"))
        worker = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(worker)
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
                'test -f .mediawiki/fixture\n"$MW_BIN"\n'
            )
            (repo / "tests/test-selinux-traefik.sh").write_text(
                'test -z "${MWCLI_CI_MEDIAWIKI_SNAPSHOT:-}"\n'
                'test ! -e .mediawiki\n'
                'test -z "${GITLAB_STATUS_TOKEN:-}"\n'
            )
            git("add", ".")
            git("commit", "-m", "Fixture")
            sha = git("rev-parse", "HEAD")
            media = root / "mediawiki"
            media.mkdir()
            (media / "fixture").write_text("clean snapshot")
            for test, code, binary_name in (
                ("docker-general", 0, None),
                ("docker-general", 23, "wm"),
                ("selinux-traefik", 0, None),
            ):
                with self.subTest(test=test, exit_code=code, binary_name=binary_name):
                    work = root / f"{test}-{code}"
                    inputs = work / "inputs"
                    inputs.mkdir(parents=True)
                    git("bundle", "create", str(inputs / "source.bundle"), "HEAD")
                    plan = {
                        "tested_sha": sha, "pipeline_id": 23, "request_id": 19,
                    }
                    if binary_name is not None:
                        plan["binary_name"] = binary_name
                    (inputs / "plan.json").write_text(json.dumps(plan))
                    (inputs / "mw").write_text(f"#!/bin/sh\nexit {code}\n")
                    snapshot = work / "snapshot.tar.gz"
                    if test == "docker-general":
                        with tarfile.open(snapshot, "w:gz") as archive:
                            archive.add(media, arcname=".mediawiki")
                        checksum = hashlib.sha256(snapshot.read_bytes()).hexdigest()
                        path = f"/projects/16/packages/generic/mediawiki-ci-snapshot/2026-44-{'a' * 64}/{checksum}.tar.gz"
                        (inputs / "mediawiki-snapshot.txt").write_text(f"{path} {checksum}\n")
                    source = work / "source"
                    subprocess.run(["git", "clone", str(inputs / "source.bundle"), str(source)],
                                   check=True, capture_output=True)
                    env = {key: value for key, value in os.environ.items() if key != "GITLAB_STATUS_TOKEN"}
                    with chdir(source), patch.dict(os.environ, env, clear=True), patch.object(
                        worker.sys, "argv", ["worker", test]
                    ), patch.object(worker.GitLab, "request", side_effect=lambda path, destination:
                                    destination.write_bytes(snapshot.read_bytes())):
                        self.assertEqual(worker.main(), code)
                    self.assertTrue((source / "bin" / (binary_name or "mw")).is_file())


if __name__ == "__main__":
    unittest.main()
