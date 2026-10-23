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
        self.request_name = native.REQUEST_JOB
        self.gate_status = "running"
        self.gate_user = 7
        self.merge_bundle = None
        self.sha = SHA
        self.matrix = []
        self.test_plan = None

    def get(self, path):
        if path == "/projects/16/jobs/19":
            return {"id": 19, "name": self.request_name, "status": "success",
                    "pipeline": {"id": 23}, "commit": {"id": self.sha},
                    "artifacts_file": {"filename": "artifacts.zip"} if self.test_plan is not None else {}}
        if path == "/projects/16/jobs/19/artifacts/integration-plan.json":
            return self.test_plan
        if path == "/projects/16/pipelines/23":
            return {"id": 23, "sha": self.sha, "status": "canceled" if self.canceled else "manual"}
        raise AssertionError(path)

    def all(self, path):
        if "/statuses?" in path:
            return self.status_records
        if path == "/projects/16/pipelines/23/jobs":
            jobs = [
                {"id": self.request_id, "name": self.request_name, "status": "success"},
                {"id": 20, "name": native.GATE_JOB, "status": self.gate_status, "user": {"id": self.gate_user}},
                {"id": 17, "name": "build", "status": "success"},
                {"id": 18, "name": "prepare-mediawiki", "status": "success"},
            ]
            if self.merge_bundle is not None:
                jobs.append({"id": 15, "name": "prepare-merge", "status": "success"})
            if self.matrix:
                jobs = [job for job in jobs if job["name"] != native.GATE_JOB] + self.matrix
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

    def test_posts_test_results_and_summary_without_restarting_a_running_gate(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
        self.assertEqual(len(api.posted), 5)
        for path, body in api.posted[:4]:
            self.assertEqual(path, "/projects/16/statuses/" + SHA)
            self.assertEqual(body["pipeline_id"], 23)
            self.assertEqual(body["description"], native.description(19, 123, 1, SHA))
        self.assertEqual(api.posted[3][1]["state"], "success")

    def test_native_failure_leaves_the_waiting_job_to_observe_failure(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results("repoclient")):
            native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
        self.assertEqual(len(api.posted), 5)
        self.assertEqual(api.posted[3][1]["state"], "failed")
        self.assertTrue(all("/statuses/" in path for path, _ in api.posted))

    def test_missing_native_job_is_reported_as_failed(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value={}):
            native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
        self.assertEqual([body["state"] for _, body in api.posted], ["failed"] * (len(native.TESTS) + 1))

    def test_failed_preparation_cannot_reuse_old_successes(self):
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results()) as fetch:
            native.finish(api, 19, SHA, 123, 2, prepared=False, jobs_succeeded=True)
        fetch.assert_not_called()
        self.assertEqual([body["state"] for _, body in api.posted], ["failed"] * (len(native.TESTS) + 1))

    def test_explicit_rerun_retries_failed_gate(self):
        api = FakeGitLab()
        api.gate_status = "failed"
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 2, jobs_succeeded=True)
        self.assertEqual(api.posted[-1], ("/projects/16/jobs/20/retry", {}))
        self.assertIn("attempt=2", api.posted[0][1]["description"])

    def test_canceled_superseded_and_passed_requests_do_not_post_or_play(self):
        for field, value in (("canceled", True), ("request_id", 21), ("gate_status", "success")):
            with self.subTest(field=field):
                api = FakeGitLab()
                setattr(api, field, value)
                native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
                self.assertEqual(api.posted, [])

    def test_cancel_during_reporting_does_not_restart_gate(self):
        api = FakeGitLab()
        original = api.post
        def cancel_after_post(path, data):
            api.canceled = True
            return original(path, data)
        api.post = cancel_after_post
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
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
            self.assertEqual(len(api.posted), len(native.TESTS) + 1)
            self.assertEqual(api.posted[-1][1]["name"], native.SUMMARY_CONTEXT)
            self.assertTrue(all(body["state"] == "pending" for _, body in api.posted))

    def test_new_workflow_jobs_are_discovered_without_a_test_registry_entry(self):
        jobs = {"jobs": [
            {"name": "Integration (windows-amd64/new-update) / Test", "conclusion": "success",
             "html_url": f"https://github.com/{native.REPOSITORY}/actions/runs/123/job/99"},
            {"name": "Report to GitLab", "conclusion": None, "html_url": "unused"},
        ]}
        with patch.dict(os.environ, {"GH_TOKEN": "fixture"}), patch.object(
            native.urllib.request, "urlopen", return_value=io.BytesIO(json.dumps(jobs).encode())
        ):
            results = native.github_results(123, 1)
        self.assertEqual(list(results), ["windows-amd64/new-update"])
        api = FakeGitLab()
        api.test_plan = [{"platform": "windows-amd64", "test": "new-update"}]
        with patch.object(native, "github_results", return_value=results):
            native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
        self.assertEqual(api.posted[-1][1]["name"], native.SUMMARY_CONTEXT)
        self.assertEqual(api.posted[-1][1]["state"], "success")

    def test_preparation_expands_the_single_request_artifact_without_github_main_test_entries(self):
        api = FakeGitLab()
        api.test_plan = [{"platform": "windows-amd64", "test": "future-test"}]
        def source(api, plan, jobs, directory):
            plan["tested_sha"] = SHA
        with tempfile.TemporaryDirectory() as temporary, patch.object(
            native, "prepare_source", side_effect=source
        ), patch.object(native, "output") as output, patch.dict(os.environ, {
            "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1",
        }):
            directory = Path(temporary) / "inputs"
            native.prepare(api, 19, directory)
            plan = json.loads((directory / "plan.json").read_text())
            self.assertEqual(plan["tests"], [{"platform": "windows-amd64", "test": "future-test",
                                              "name": "windows-amd64/future-test"}])
            values = dict(call.args for call in output.call_args_list)
            self.assertEqual(json.loads(values["test_matrix"]), {"include": plan["tests"]})
        self.assertEqual({body["name"] for _, body in api.posted},
                         {"github/windows-amd64/future-test", native.SUMMARY_CONTEXT})

    def test_request_plan_selects_new_platforms_and_missing_tests_fail_closed(self):
        api = FakeGitLab()
        api.test_plan = [
            {"platform": "windows-amd64", "test": "new-update"},
            {"platform": "linux-arm64", "test": "holesail"},
        ]
        jobs = {job["name"]: job for job in api.all("/projects/16/pipelines/23/jobs")}
        plan = native.test_plan(api, native.load_request(api, 19)[0], jobs)
        self.assertEqual({test["name"] for test in plan}, {"windows-amd64/new-update", "linux-arm64/holesail"})
        with patch.object(native, "github_results", return_value={"windows-amd64/new-update": {"state": "success"}}):
            native.finish(api, 19, SHA, 123, 1, jobs_succeeded=True)
        self.assertEqual(api.posted[-1][1]["state"], "failed")
        self.assertIn(("github/linux-arm64/holesail", "failed"),
                      [(body["name"], body["state"]) for _, body in api.posted])

    def test_failed_or_skipped_workflow_dependencies_cannot_pass_summary(self):
        for result in ("failure", "skipped", "cancelled", None):
            needs = {"prepare": {"result": "success"}, "integration": {"result": "success"},
                     "native": {"result": result}}
            self.assertFalse(native.workflow_jobs_succeeded(json.dumps(needs)))
        self.assertFalse(native.workflow_jobs_succeeded("{}"))
        self.assertTrue(native.workflow_jobs_succeeded(json.dumps({
            "prepare": {"result": "success"}, "integration": {"result": "success"},
            "native": {"result": "success"}})))
        api = FakeGitLab()
        with patch.object(native, "github_results", return_value=self.results()):
            native.finish(api, 19, SHA, 123, 1)
        self.assertEqual(api.posted[-1][1]["state"], "failed")



class PlatformCallbackTests(unittest.TestCase):
    def setUp(self):
        environment = patch.dict(os.environ, {
            "INTEGRATION_PLATFORM": "linux-arm64", "TEST_NAME": "general",
            "REQUEST_JOB_NAME": "integration-github-linux-arm64-general-request",
        })
        environment.start()
        self.addCleanup(environment.stop)
        self.api = FakeGitLab()
        self.api.request_name = os.environ["REQUEST_JOB_NAME"]

    def test_platform_failure_reports_after_the_default_gate_has_passed(self):
        self.api.gate_status = "success"
        context = "github/linux-arm64/general"
        with patch.object(native, "github_results", return_value={context: {"state": "failed"}}):
            native.finish_platform(self.api, 19, SHA, 123, 1)
        self.assertEqual(len(self.api.posted), 1)
        self.assertEqual(self.api.posted[0][1]["name"], context)
        self.assertEqual(self.api.posted[0][1]["state"], "failed")

    def test_shell_selection_reuses_existing_docker_test_names(self):
        with patch.dict(os.environ, {
            "TEST_NAME": "all-databases",
            "REQUEST_JOB_NAME": "integration-github-linux-arm64-all-databases-request",
        }):
            self.assertEqual(native.platform_identity()[2], "test-docker-mw-all-dbs.sh")
        self.assertEqual(native.platform_identity()[2], "test-general.sh")

    def test_preparation_exports_the_selected_shell_script(self):
        original = self.api.all
        def jobs(path):
            return original(path) + [{"name": name, "status": "success"} for name in ("checks", "test")]
        def source(api, plan, jobs, directory):
            plan["tested_sha"] = SHA
        with tempfile.TemporaryDirectory() as temporary, patch.object(
            self.api, "all", side_effect=jobs
        ), patch.object(native, "prepare_source", side_effect=source), patch.object(
            native, "git"
        ) as git, patch.object(native, "output") as output, patch.dict(
            os.environ, {"GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1"}
        ):
            directory = Path(temporary) / "inputs"
            native.prepare_platform(self.api, 19, directory)
            git.assert_called_once_with("cat-file", "-e", f"{SHA}:tests/test-general.sh",
                                        cwd=directory / "repository.git")
            self.assertIn(unittest.mock.call("test_script", "test-general.sh"), output.call_args_list)
        self.assertEqual(self.api.posted[0][1]["name"], "github/linux-arm64/general")
        self.assertEqual(self.api.posted[0][1]["state"], "pending")

    def test_paths_arguments_and_unrelated_request_names_are_rejected(self):
        for change in ({"TEST_NAME": "../secret"}, {"TEST_NAME": "general --flag"},
                       {"REQUEST_JOB_NAME": "build"}, {"INTEGRATION_PLATFORM": "other"}):
            with self.subTest(change=change), patch.dict(os.environ, change), self.assertRaises(ValueError):
                native.platform_identity()

    def test_canceled_or_superseded_platform_requests_cannot_report(self):
        for field, value in (("canceled", True), ("request_id", 21), ("request_name", "build")):
            api = FakeGitLab()
            api.request_name = os.environ["REQUEST_JOB_NAME"]
            setattr(api, field, value)
            with self.subTest(field=field), patch.object(native, "github_results") as fetch:
                native.finish_platform(api, 19, SHA, 123, 1)
                fetch.assert_not_called()
                self.assertEqual(api.posted, [])

    def test_failed_preparation_does_not_reuse_old_platform_successes(self):
        with patch.object(native, "github_results") as fetch:
            native.finish_platform(self.api, 19, "", 123, 1, prepared=False)
        fetch.assert_not_called()
        self.assertEqual(self.api.posted[0][1]["state"], "failed")

    def test_platform_result_is_read_individually_across_reporting_reruns(self):
        def response(request, timeout):
            jobs = [] if "/attempts/2/" in request.full_url else [
                {"name": "Test", "conclusion": "success", "html_url": "https://example.test/log"}]
            return io.BytesIO(json.dumps({"jobs": jobs}).encode())
        context = "github/linux-arm64/general"
        with patch.dict(os.environ, {"GH_TOKEN": "test-token"}), patch.object(
            native.urllib.request, "urlopen", side_effect=response
        ):
            self.assertEqual(native.github_results(123, 2, {context: "Test"})[context]["state"], "success")


if __name__ == "__main__":
    unittest.main()
