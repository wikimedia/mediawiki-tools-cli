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
        self.gate_status = "manual"
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
    def results(self, failure=None):
        return {name: {"state": "failed" if name == failure else "success",
                       "url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i}"}
                for i, name in enumerate(native.TESTS, 1)}

    def test_posts_four_independent_results_to_exact_pipeline_then_plays_gate(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results("repoclient")):
            native.finish(api, 19, SHA, 123, 1)
        self.assertEqual(len(api.posted), 5)
        for path, body in api.posted[:4]:
            self.assertEqual(path, "/projects/16/statuses/" + SHA)
            self.assertEqual(body["pipeline_id"], 23)
            self.assertEqual(body["description"], native.description(19, 123, 1, SHA))
        self.assertEqual(api.posted[3][1]["state"], "failed")
        self.assertEqual(api.posted[-1][0], "/projects/16/jobs/20/play")
        receipt = json.loads(api.posted[-1][1]["job_variables_attributes"][0]["value"])
        self.assertEqual(receipt["pipeline_id"], 23)
        self.assertEqual(len(receipt["records"]), 4)

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
        self.assertEqual(api.posted[-1][0], "/projects/16/jobs/20/play")
        receipt = json.loads(api.posted[-1][1]["job_variables_attributes"][0]["value"])
        self.assertTrue(all("attempt=2" in record["description"] for record in receipt["records"]))
        self.assertIn("attempt=2", api.posted[0][1]["description"])

    def test_manual_gate_attempt_by_another_user_requires_a_new_pipeline(self):
        api = FakeGitLab()
        api.gate_status = "failed"
        api.gate_user = 99
        with patch.object(native, "github_results", return_value=self.results()):
            with self.assertRaisesRegex(ValueError, "manually started"):
                native.finish(api, 19, SHA, 123, 2)
        self.assertEqual(len(api.posted), 4)

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
        self.assertEqual(len(api.posted), 4)

    def test_failed_job_rerun_keeps_previous_successful_job_conclusions(self):
        def response(request, timeout):
            attempt = 2 if "/attempts/2/" in request.full_url else 1
            tests = ["repoclient"] if attempt == 2 else list(native.TESTS)
            return io.BytesIO(json.dumps({"jobs": [
                {"name": f"Native integration ({name}) / Test", "conclusion": "success",
                 "html_url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/{i}"}
                for i, name in enumerate(tests, 1)
            ]}).encode())
        with patch.dict(os.environ, {"GH_TOKEN": "test-token"}), patch.object(
            native.urllib.request, "urlopen", side_effect=response
        ):
            self.assertEqual(set(native.github_results(123, 2)), set(native.TESTS))

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
