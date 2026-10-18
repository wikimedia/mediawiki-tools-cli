"""Platform callbacks must remain independent of the default integration gate."""

import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("platform_ci", Path(__file__).with_name("gitlab-platform.py"))
platform_ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(platform_ci)
SHA = "a" * 40
REQUEST = "integration-github-linux-arm64-platform-filesystem-request"
ENV = {"INTEGRATION_PLATFORM": "linux-arm64", "TEST_NAME": "platform-filesystem",
       "REQUEST_JOB_NAME": REQUEST, "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1"}


class FakeGitLab:
    def __init__(self):
        self.posted = []
        self.canceled = False
        self.current_request = 19
        self.request_name = REQUEST

    def get(self, path):
        if path == "/projects/16/jobs/19":
            return {"id": 19, "name": self.request_name, "status": "success",
                    "pipeline": {"id": 23}, "commit": {"id": SHA}}
        if path == "/projects/16/pipelines/23":
            return {"id": 23, "sha": SHA, "status": "canceled" if self.canceled else "success"}
        raise AssertionError(path)

    def all(self, path):
        if path == "/projects/16/pipelines/23/jobs":
            return [{"id": self.current_request, "name": REQUEST, "status": "success"},
                    # Default Ubuntu integration has already finished.
                    {"id": 20, "name": "integration-github", "status": "success"},
                    *({"id": i, "name": name, "status": "success"} for i, name in
                      enumerate(("checks: [lint]", "test", "build"), 30))]
        raise AssertionError(path)

    def post(self, path, data):
        self.posted.append((path, data))


class PlatformTests(unittest.TestCase):
    def setUp(self):
        self.environment = patch.dict(os.environ, ENV)
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_late_platform_failure_is_reported_after_default_gate_passes(self):
        api = FakeGitLab()
        with patch.object(platform_ci, "test_result", return_value=("failed", "https://example.test/log")):
            platform_ci.finish(api, 19, SHA, True)
        self.assertEqual(len(api.posted), 1)
        path, body = api.posted[0]
        self.assertEqual(path, "/projects/16/statuses/" + SHA)
        self.assertEqual(body["name"], "github/linux-arm64/platform-filesystem")
        self.assertEqual(body["state"], "failed")
        self.assertEqual(body["description"], platform_ci.native.description(19, 123, 1, SHA))

    def test_each_platform_and_test_has_its_own_request_and_result(self):
        for target in ("linux-arm64", "macos-arm64", "windows-amd64"):
            with self.subTest(target=target), patch.dict(os.environ, {
                "INTEGRATION_PLATFORM": target, "TEST_NAME": "docker-example",
                "REQUEST_JOB_NAME": f"integration-github-{target}-docker-example-request",
            }):
                self.assertEqual(platform_ci.identity()[3], f"github/{target}/docker-example")

    def test_canceled_superseded_and_unrelated_requests_cannot_post(self):
        for field, value in (("canceled", True), ("current_request", 21), ("request_name", "build")):
            api = FakeGitLab()
            setattr(api, field, value)
            with self.subTest(field=field), patch.object(platform_ci, "test_result") as fetch:
                platform_ci.finish(api, 19, SHA, True)
                fetch.assert_not_called()
                self.assertEqual(api.posted, [])

    def test_invalid_selector_or_mismatched_request_is_rejected(self):
        for change in ({"TEST_NAME": "../secret"}, {"TEST_NAME": "x --flag"},
                       {"INTEGRATION_PLATFORM": "other"}, {"REQUEST_JOB_NAME": "build"}):
            with self.subTest(change=change), patch.dict(os.environ, change), self.assertRaises(ValueError):
                platform_ci.identity()

    def test_failed_preparation_cannot_reuse_a_previous_test_success(self):
        api = FakeGitLab()
        with patch.object(platform_ci, "test_result") as fetch:
            platform_ci.finish(api, 19, "", False)
        fetch.assert_not_called()
        self.assertEqual(api.posted[0][1]["state"], "failed")

    def test_prepares_only_source_and_the_selected_test_plan(self):
        api = FakeGitLab()
        def source(_api, plan, _jobs, directory):
            plan["tested_sha"] = SHA
            (directory / "source.bundle").write_bytes(b"fixture")
        with tempfile.TemporaryDirectory() as temporary, patch.object(
            platform_ci.native, "prepare_source", side_effect=source
        ), patch.object(platform_ci.native, "output"):
            directory = Path(temporary) / "inputs"
            platform_ci.prepare(api, 19, directory)
            plan = json.loads((directory / "plan.json").read_text())
            self.assertEqual(plan["test"], "platform-filesystem")
            self.assertEqual(plan["platform"], "linux-arm64")
            self.assertEqual(set(p.name for p in directory.iterdir()), {"plan.json", "source.bundle"})
        self.assertEqual(api.posted[0][1]["state"], "pending")

    def test_reporting_rerun_recovers_the_previous_attempts_test_job(self):
        def response(request, timeout):
            jobs = [] if "/attempts/2/" in request.full_url else [
                {"name": "Test", "conclusion": "success", "html_url": "https://example.test/log"}]
            return io.BytesIO(json.dumps({"jobs": jobs}).encode())
        with patch.dict(os.environ, {"GITHUB_RUN_ATTEMPT": "2", "GH_TOKEN": "test-token"}), patch.object(
            platform_ci.urllib.request, "urlopen", side_effect=response
        ):
            self.assertEqual(platform_ci.test_result(), ("success", "https://example.test/log"))


if __name__ == "__main__":
    unittest.main()
