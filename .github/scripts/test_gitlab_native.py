"""Exercise callback, retry and snapshot handling without changing either service."""

import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("native", Path(__file__).with_name("gitlab-native.py"))
native = importlib.util.module_from_spec(spec)
spec.loader.exec_module(native)
SHA = "a" * 40


class FakeGitLab:
    def __init__(self):
        self.posted = []
        self.status_records = []
        self.canceled = False
        self.request_id = 19
        self.gate_status = "running"
        self.gate_user = 7
        self.merge_bundle = None
        self.sha = SHA

    def get(self, path):
        if path == "/projects/16/jobs/19":
            return {"id": 19, "name": native.REQUEST_JOB, "status": "success",
                    "pipeline": {"id": 23}, "commit": {"id": self.sha}}
        if path == "/projects/16/pipelines/23":
            return {"id": 23, "sha": self.sha, "status": "canceled" if self.canceled else "manual"}
        raise AssertionError(path)

    def all(self, path):
        if "/statuses?" in path:
            return self.status_records
        if path == "/projects/16/pipelines/23/jobs":
            jobs = [
                {"id": self.request_id, "name": native.REQUEST_JOB, "status": "success"},
                {"id": 20, "name": native.GATE_JOB, "status": self.gate_status, "user": {"id": self.gate_user}},
                {"id": 17, "name": "build", "status": "success"},
                {"id": 18, "name": "prepare-mediawiki", "status": "success"},
            ]
            if self.merge_bundle is not None:
                jobs.append({"id": 15, "name": "prepare-merge", "status": "success"})
            return jobs
        raise AssertionError(path)

    def request(self, path, destination):
        data = self.merge_bundle if path.endswith(".ci-merge.bundle") else b"fixture"
        Path(destination).write_bytes(data)

    def post(self, path, data):
        self.posted.append((path, data))
        if "/statuses/" in path:
            self.status_records.append({
                **data, "id": len(self.status_records) + 1,
                "status": data["state"], "author": {"id": 7},
            })
        return {}


class CallbackTests(unittest.TestCase):
    def test_dispatched_request_must_finish_successfully_before_testing(self):
        api = FakeGitLab()
        original = api.get
        for terminal in ("success", "failed", "canceled"):
            calls = iter(["running", "running", terminal])
            def get(path):
                job = original(path)
                if path == "/projects/16/jobs/19":
                    job["status"] = next(calls)
                return job
            with self.subTest(terminal=terminal), patch.object(api, "get", side_effect=get), patch.object(
                native.time, "sleep"
            ) as sleep:
                result = native.load_request(api, 19, wait=True)
                self.assertEqual(result is not None, terminal == "success")
                self.assertEqual(sleep.call_count, 2)

    def test_stuck_request_times_out_instead_of_silently_skipping_tests(self):
        api = FakeGitLab()
        job = api.get("/projects/16/jobs/19")
        job["status"] = "running"
        with patch.object(api, "get", return_value=job), patch.object(
            native.time, "monotonic", side_effect=[0, 361]
        ), self.assertRaisesRegex(ValueError, "did not finish"):
            native.load_request(api, 19, wait=True)

    def results(self, failure=None):
        return {name: {"state": "failed" if name == failure else "success",
                       "url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i}"}
                for i, name in enumerate(native.TESTS, 1)}

    def test_posts_existing_results_and_missing_platform_smoke_as_failure(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 1)
        self.assertEqual(len(api.posted), 5)
        for path, body in api.posted[:4]:
            self.assertEqual(path, "/projects/16/statuses/" + SHA)
            self.assertEqual(body["pipeline_id"], 23)
            self.assertEqual(body["description"], native.description(19, 123, 1, SHA))
        self.assertEqual(api.posted[3][1]["state"], "success")
        self.assertEqual(api.posted[4][1]["name"], "github/platform-smoke")
        self.assertEqual(api.posted[4][1]["state"], "failed")

    def test_native_failure_leaves_the_waiting_job_to_observe_failure(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results("repoclient")):
            native.finish(api, 19, SHA, 123, 1)
        self.assertEqual(len(api.posted), 5)
        self.assertEqual(api.posted[3][1]["state"], "failed")
        self.assertTrue(all("/statuses/" in path for path, _ in api.posted))

    def test_missing_native_job_is_reported_as_failed(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value={}):
            native.finish(api, 19, SHA, 123, 1)
        self.assertEqual([body["state"] for _, body in api.posted[:4]], ["failed"] * 4)

    def test_failed_preparation_cannot_reuse_old_successes(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results()) as fetch:
            native.finish(api, 19, SHA, 123, 2, prepared=False)
        fetch.assert_not_called()
        self.assertEqual([body["state"] for _, body in api.posted[:4]], ["failed"] * 4)

    def test_explicit_rerun_retries_failed_gate(self):
        api = FakeGitLab()
        api.gate_status = "failed"
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 2)
        self.assertEqual(api.posted[-1], ("/projects/16/jobs/20/retry", {}))
        self.assertIn("attempt=2", api.posted[0][1]["description"])

    def test_canceled_superseded_and_passed_requests_do_not_post_or_play(self):
        for field, value in (("canceled", True), ("request_id", 21), ("gate_status", "success")):
            with self.subTest(field=field):
                api = FakeGitLab()
                setattr(api, field, value)
                native.finish(api, 19, SHA, 123, 1)
                self.assertEqual(api.posted, [])

    def test_cancel_during_reporting_does_not_restart_gate(self):
        api = FakeGitLab()
        original = api.post
        def cancel_after_post(path, data):
            api.canceled = True
            return original(path, data)
        api.post = cancel_after_post
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 1)
        self.assertEqual(len(api.posted), 5)

    def test_failed_job_rerun_keeps_previous_successful_job_conclusions(self):
        def response(request, timeout):
            attempt = 2 if "/attempts/2/" in request.full_url else 1
            tests = ["repoclient"] if attempt == 2 else list(native.TESTS)
            return io.BytesIO(json.dumps({"jobs": [
                {"name": f"Integration ({name}) / Test", "conclusion": "success",
                 "html_url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i}"}
                for i, name in enumerate(tests, 1)
            ]}).encode())
        with patch.dict(os.environ, {"GH_TOKEN": "test-token"}), patch.object(
            native.urllib.request, "urlopen", side_effect=response
        ):
            self.assertEqual(set(native.github_results(123, 2)), set(native.TESTS))

    def test_platform_smoke_is_aggregated_from_all_three_native_runners(self):
        def response(request, timeout):
            return io.BytesIO(json.dumps({"jobs": [
                {"name": f"Integration ({name}) / Test", "conclusion": "success",
                 "html_url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i}"}
                for i, name in enumerate(native.TESTS, 1)
            ] + [
                {"name": f"Integration (platform-smoke) / Test ({label})", "conclusion": "success",
                 "html_url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i + 10}"}
                for i, label in enumerate(native.PLATFORM_SMOKE_LABELS, 1)
            ]}).encode())
        with patch.dict(os.environ, {"GH_TOKEN": "test-token"}), patch.object(
            native.urllib.request, "urlopen", side_effect=response
        ):
            results = native.github_results(123, 1)
        self.assertEqual(results["platform-smoke"]["state"], "success")
        self.assertEqual(set(native.REPORTS), set(native.TESTS) | {"platform-smoke"})

    def test_platform_smoke_requires_every_runner_to_succeed(self):
        def response(request, timeout):
            return io.BytesIO(json.dumps({"jobs": [
                {"name": f"Integration (platform-smoke) / Test ({label})",
                 "conclusion": "failure" if label == "Windows x64" else "success",
                 "html_url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i}"}
                for i, label in enumerate(native.PLATFORM_SMOKE_LABELS, 1)
            ]}).encode())
        with patch.dict(os.environ, {"GH_TOKEN": "test-token"}), patch.object(
            native.urllib.request, "urlopen", side_effect=response
        ):
            results = native.github_results(123, 1)
        self.assertEqual(results["platform-smoke"]["state"], "failed")

    def test_downloads_and_verifies_the_exact_merged_bundle(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo = root / "repo"
            repo.mkdir()
            def git(*args):
                return subprocess.check_output(["git", *args], cwd=repo, text=True,
                                               stderr=subprocess.DEVNULL).strip()
            git("init")
            git("config", "user.email", "ci@example.invalid")
            git("config", "user.name", "CI test")
            (repo / "source").write_text("source\n")
            git("add", ".")
            git("commit", "-m", "Source")
            api = FakeGitLab()
            api.sha = git("rev-parse", "HEAD")
            (repo / "target").write_text("merged target\n")
            git("add", ".")
            git("commit", "-m", "Merged target")
            tested_sha = git("rev-parse", "HEAD")
            git("bundle", "create", str(root / "snapshot.bundle"), "HEAD")
            api.merge_bundle = (root / "snapshot.bundle").read_bytes()
            with patch.dict(os.environ, {"GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1"}), patch.object(native, "output"):
                native.prepare(api, 19, root / "inputs")
            plan = json.loads((root / "inputs/plan.json").read_text())
            self.assertEqual(plan["source_sha"], api.sha)
            self.assertEqual(plan["tested_sha"], tested_sha)
            self.assertEqual(len(api.posted), 4)
            self.assertEqual([body["state"] for _, body in api.posted], ["pending"] * 4)


if __name__ == "__main__":
    unittest.main()
