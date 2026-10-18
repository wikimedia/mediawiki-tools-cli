"""The VM reader must not accept forged identities or unrelated result batches."""
import copy
import importlib.util
import io
from pathlib import Path
import unittest
from unittest.mock import patch

from github_status import CONTEXTS, description

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
            for i, name in enumerate(CONTEXTS, 1)]


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
                       {"description": description(19, 123, 2, SHA)},
                       {"description": description(19, 123, 1, "b" * 40)},
                       {"status": "pending"}, {"allow_failure": True},
                       {"target_url": "https://github.com/other/repo/actions/runs/123"}):
            with self.subTest(change=change):
                values = records()
                values[0].update(change)
                self.assertEqual(self.result(values=values)["state"], "pending")
        self.assertEqual(self.result(values=records()[:-1])["state"], "pending")

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
        for authorized in (True, False):
            with self.subTest(authorized=authorized), tempfile.TemporaryDirectory() as directory:
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
                            reader, "check", return_value={"state": "success", "message": "All passed"}
                        ) as check, contextlib.redirect_stdout(output):
                            self.assertEqual(client.main(), 0 if authorized else 1)
                            self.assertEqual(check.call_count, 1 if authorized else 0)
                        self.assertNotIn("ephemeral-test-job-token", output.getvalue())
                    finally:
                        server.shutdown()
                        worker.join(timeout=2)


if __name__ == "__main__":
    unittest.main()
