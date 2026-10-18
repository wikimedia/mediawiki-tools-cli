"""The VM reader must not accept forged identities or unrelated result batches."""
import copy
import importlib.util
import io
import json
import tempfile
import threading
from types import SimpleNamespace
from pathlib import Path
import unittest
from unittest.mock import patch

from github_status import SUMMARY_CONTEXT, description

spec = importlib.util.spec_from_file_location("reader", Path(__file__).with_name("wait-status.py"))
reader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reader)
SHA = "a" * 40
CONFIG = {"bot_id": 7, "repository": "wikimedia/mediawiki-tools-cli"}
JOB = {"id": 20, "name": "integration-github", "status": "running",
       "pipeline": {"id": 23, "project_id": 16}}


def records():
    return [{"id": i, "name": name, "status": "success", "author": {"id": 7},
             "description": description(19, 123, 1, SHA),
             "target_url": f"https://github.com/{CONFIG['repository']}/actions/runs/123/job/{i}"}
            for i, name in enumerate((SUMMARY_CONTEXT,), 1)]


class ReaderTests(unittest.TestCase):
    def result(self, *, values=None, job=None, jobs=None, pipeline=None):
        class API:
            def get(self, _):
                return pipeline or {"sha": SHA, "status": "running"}
        with patch.object(reader, "pipeline_jobs", return_value=jobs or {
            "integration-github": {"id": 20},
            "integration-github-request": {"id": 19, "status": "success"},
        }), patch.object(reader, "statuses", return_value=records() if values is None else values):
            return reader.check(API(), JOB if job is None else job, CONFIG)

    def test_complete_authentic_batch_passes(self):
        self.assertEqual(self.result()["state"], "success")

    def test_forged_old_partial_and_pending_results_cannot_pass(self):
        for change in ({"author": {"id": 99}}, {"description": description(18, 123, 1, SHA)},
                       {"description": description(19, 123, 1, "b" * 40)},
                       {"status": "pending"}, {"allow_failure": True},
                       {"target_url": "https://github.com/other/repo/actions/runs/123"}):
            with self.subTest(change=change):
                values = records()
                values[0].update(change)
                self.assertEqual(self.result(values=values)["state"], "pending")
        self.assertEqual(self.result(values=records()[:-1])["state"], "pending")

    def test_individual_results_do_not_define_or_override_the_workflow_outcome(self):
        values = records()
        values.extend({**values[0], "id": index + 10, "name": f"github/new-test-{index}", "status": "failed"}
                      for index in range(12))
        self.assertEqual(self.result(values=values)["state"], "success")
        values.append({**values[0], "id": 100, "status": "pending"})
        self.assertEqual(self.result(values=values)["state"], "pending")

    def test_matrix_gate_authenticates_its_declared_test_without_a_registry(self):
        name = "integration-github: [windows-amd64, arbitrary-new-test]"
        job = {**JOB, "name": name}
        jobs = {name: {"id": 20}, "integration-github-request": {"id": 19, "status": "success"}}
        values = records()
        result = {**values[0], "id": 2, "name": "github/windows-amd64/arbitrary-new-test"}
        values.append(result)
        self.assertEqual(self.result(values=values, job=job, jobs=jobs)["state"], "success")
        for change in ({"author": {"id": 99}}, {"description": description(19, 123, 2, SHA)},
                       {"status": "pending"}, {"allow_failure": True}):
            with self.subTest(change=change):
                self.assertEqual(self.result(values=[values[0], {**result, **change}], job=job, jobs=jobs)["state"], "pending")

    def test_authenticated_failure_is_terminal(self):
        values = records()
        values[0]["status"] = "failed"
        self.assertEqual(self.result(values=values)["state"], "failed")

    def test_wrong_project_job_or_canceled_identity_is_rejected(self):
        for change in ({"name": "other"}, {"status": "canceled"},
                       {"pipeline": {"id": 23, "project_id": 99}}, {"id": 21}):
            job = {**copy.deepcopy(JOB), **change}
            with self.subTest(change=change), self.assertRaises(PermissionError):
                self.result(job=job)
        with self.assertRaises(PermissionError):
            self.result(pipeline={"sha": SHA, "status": "canceled"})

    def test_reads_head_from_bounded_bundle_header(self):
        stream = io.BytesIO(b"# v2 git bundle\n" + SHA.encode() + b" HEAD\n\nPACKdata")
        self.assertEqual(reader.bundle_head(stream), SHA)
        self.assertEqual(stream.read(), b"\nPACKdata")
        for data in (b"bad\n", b"# v2 git bundle\n\nPACKdata", b"# v3 git bundle\n" + b"x" * 100000):
            with self.subTest(data=data[:20]), self.assertRaises(ValueError):
                reader.bundle_head(io.BytesIO(data))


class DispatchTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.job = {**JOB, "id": 19, "name": reader.REQUEST_JOB}
        self.jobs = {name: {"status": "success"} for name in
                     ("checks: [lint]", "checks: [vet]", "test", "build", "prepare-mediawiki")}
        self.jobs.update({reader.REQUEST_JOB: {"id": 19}, reader.GATE_JOB: {"id": 20}})
        self.pipeline = {"status": "running", "ref": "main"}
        self.api = SimpleNamespace(get=lambda _: self.pipeline)
        self.server = SimpleNamespace(api=self.api, config=CONFIG,
                                      state=Path(self.directory.name),
                                      dispatch_lock=threading.Lock(), dispatch_token="test-secret")

    def launch(self, job=None):
        with patch.object(reader, "pipeline_jobs", return_value=self.jobs):
            return reader.dispatch(self.server, self.job if job is None else job)

    def test_fixed_dispatch_is_acknowledged_once_across_restarts(self):
        response = unittest.mock.MagicMock()
        response.__enter__.return_value.status = 204
        with patch.object(reader.urllib.request, "build_opener") as opener:
            opener.return_value.open.return_value = response
            self.assertEqual(self.launch()["state"], "success")
            self.server.dispatch_lock = threading.Lock()  # Simulate service restart.
            self.assertEqual(self.launch()["state"], "success")
            opener.return_value.open.assert_called_once()
            request = opener.return_value.open.call_args.args[0]
            self.assertEqual(request.full_url,
                             "https://api.github.com/repos/wikimedia/mediawiki-tools-cli/"
                             "actions/workflows/gitlab-integration.yml/dispatches")
            self.assertEqual(json.loads(request.data), {"ref": "main", "inputs": {
                "project_id": "16", "job_id": "19", "job_name": reader.REQUEST_JOB,
                "job_status": "success", "run_label": "main"}})
            self.assertEqual(request.get_header("Authorization"), "Bearer test-secret")

    def test_dispatch_labels_use_the_pipeline_mr_or_ref(self):
        response = unittest.mock.MagicMock()
        response.__enter__.return_value.status = 204
        cases = (("refs/merge-requests/687/head", "MR !687"),
                 ("refs/merge-requests/687/merge", "MR !687"),
                 ("main", "main"), ("v0.34.0", "v0.34.0"), ("", "pipeline 23"))
        with patch.object(reader.urllib.request, "build_opener") as opener:
            opener.return_value.open.return_value = response
            for index, (ref, label) in enumerate(cases):
                with self.subTest(ref=ref):
                    self.pipeline["ref"] = ref
                    job = {**self.job, "id": 30 + index, "ref": "untrusted-client-ref"}
                    self.jobs[reader.REQUEST_JOB]["id"] = job["id"]
                    self.assertEqual(self.launch(job)["state"], "success")
                    request = opener.return_value.open.call_args.args[0]
                    self.assertEqual(json.loads(request.data)["inputs"]["run_label"], label)

    def test_dispatch_accepts_response_with_workflow_run_details(self):
        response = unittest.mock.MagicMock()
        response.__enter__.return_value.status = 200
        with patch.object(reader.urllib.request, "build_opener") as opener:
            opener.return_value.open.return_value = response
            self.assertEqual(self.launch()["state"], "success")

    def test_unacknowledged_dispatch_can_retry(self):
        response = unittest.mock.MagicMock()
        response.__enter__.return_value.status = 204
        with patch.object(reader.urllib.request, "build_opener") as opener:
            opener.return_value.open.side_effect = [OSError("connection lost"), response]
            with self.assertRaises(OSError):
                self.launch()
            self.assertFalse(list(self.server.state.iterdir()))
            self.assertEqual(self.launch()["state"], "success")
            self.assertEqual(opener.return_value.open.call_count, 2)

    def test_rejects_unrelated_canceled_superseded_or_unprepared_requests(self):
        with patch.object(reader.urllib.request, "build_opener") as opener:
            for change in ({"name": reader.GATE_JOB}, {"status": "success"},
                           {"pipeline": {"id": 23, "project_id": 99}}, {"id": 21}):
                with self.subTest(change=change), self.assertRaises(PermissionError):
                    self.launch({**self.job, **change})
            for name in ("checks: [lint]", "checks: [vet]", "test", "build", "prepare-mediawiki"):
                self.jobs[name]["status"] = "failed"
                with self.subTest(prerequisite=name), self.assertRaises(PermissionError):
                    self.launch()
                self.jobs[name]["status"] = "success"
            self.api.get = lambda _: {"status": "canceled"}
            with self.assertRaises(PermissionError):
                self.launch()
            opener.assert_not_called()

    def test_dispatch_accepts_pipeline_matrix_jobs_and_rejects_unknown_runners(self):
        del self.jobs[reader.GATE_JOB]
        self.jobs["integration-github: [windows-amd64, arbitrary-new-test]"] = {"id": 20}
        response = unittest.mock.MagicMock()
        response.__enter__.return_value.status = 204
        with patch.object(reader.urllib.request, "build_opener") as opener:
            opener.return_value.open.return_value = response
            self.assertEqual(self.launch()["state"], "success")
            opener.return_value.open.assert_called_once()
        self.jobs["integration-github: [self-hosted, arbitrary-new-test]"] = {"id": 21}
        with self.assertRaises(ValueError):
            self.launch()

    def test_github_redirects_do_not_forward_credentials(self):
        self.assertIsNone(reader.NoRedirect().redirect_request(None, None, 302, "", {},
                                                              "https://other.example"))

class SocketTests(unittest.TestCase):
    def test_local_client_authenticates_and_observes_success_or_rejection(self):
        import contextlib
        import json
        import os
        import tempfile
        import threading
        import urllib.error
        spec = importlib.util.spec_from_file_location("client", Path(__file__).with_name("wait-github.py"))
        client = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(client)
        for authorized, launching in ((True, False), (False, False), (True, True), (False, True)):
            with self.subTest(authorized=authorized, dispatch=launching), tempfile.TemporaryDirectory() as directory:
                path = str(Path(directory) / "status.sock")
                with reader.Server(path, reader.Handler) as server:
                    server.api, server.config = object(), CONFIG
                    worker = threading.Thread(target=server.serve_forever, daemon=True)
                    worker.start()
                    def authenticate(request, timeout):
                        self.assertEqual(request.full_url, reader.API + "/job")
                        self.assertEqual(request.get_header("Job-token"), "ephemeral-test-job-token")
                        if not authorized:
                            raise urllib.error.HTTPError(request.full_url, 401, "unauthorized", {}, io.BytesIO())
                        return io.BytesIO(json.dumps(JOB).encode())
                    output = io.StringIO()
                    try:
                        with patch.object(client.LocalConnection, "socket_path", path), patch.dict(
                            os.environ, {"CI_JOB_TOKEN": "ephemeral-test-job-token"}
                        ), patch.object(reader.urllib.request, "urlopen", side_effect=authenticate), patch.object(
                            reader, "dispatch" if launching else "check", return_value={"state": "success", "message": "All passed"}
                        ) as check, contextlib.redirect_stdout(output):
                            self.assertEqual(client.main(dispatch=launching), 0 if authorized else 1)
                            self.assertEqual(check.call_count, 1 if authorized else 0)
                        self.assertNotIn("ephemeral-test-job-token", output.getvalue())
                    finally:
                        server.shutdown()
                        worker.join(timeout=2)


if __name__ == "__main__":
    unittest.main()
