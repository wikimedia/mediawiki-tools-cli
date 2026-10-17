import importlib.util
import io
import os
import tempfile
from unittest import mock
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("queue", Path(__file__).with_name("gitlab-queue.py"))
queue = importlib.util.module_from_spec(spec)
spec.loader.exec_module(queue)


class QueueTest(unittest.TestCase):
    def test_pending_job(self):
        self.assertEqual(queue.find_pending(lambda _: {"id": 1, "status": "pending", "tag_list": ["github"]}, "1"), "1")

    def test_finished_running_or_cancelled_job(self):
        for state in ["running", "success", "failed", "canceled", "manual"]:
            with self.subTest(state=state):
                self.assertEqual(queue.find_pending(lambda _: {"id": 1, "status": state, "tag_list": ["github"]}, "1"), "")

    def test_incompatible_tags(self):
        for tags in [[], ["wmcs"], ["mwcli"], ["github", "other"]]:
            with self.subTest(tags=tags):
                self.assertFalse(queue.eligible({"status": "pending", "tag_list": tags}))

    def test_invalid_input_never_makes_a_request(self):
        for job_id in ["../other", "https://example.com", "1?token=x", "0", "-1", "1\n"]:
            with self.subTest(job_id=job_id):
                with self.assertRaises(ValueError):
                    queue.find_pending(lambda _: self.fail("API called with invalid input"), job_id)

    def test_fallback_finds_job_on_later_page(self):
        urls = []
        def get(url):
            urls.append(url)
            if len(urls) == 1:
                return [{"id": i, "status": "pending", "tag_list": ["mwcli"]} for i in range(100)]
            return [{"id": 101, "status": "pending", "tag_list": ["github"]}]
        self.assertEqual(queue.find_pending(get), "101")
        self.assertTrue(urls[-1].endswith("page=2"))

    def test_outputs_selected_job_for_concurrency(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "output"
            env = {"GITLAB_API_READ_TOKEN": "fixture", "JOB_ID": "12", "GITHUB_OUTPUT": str(output)}
            response = io.StringIO('{"id":12,"status":"pending","tag_list":["github"]}')
            with mock.patch.dict(os.environ, env), mock.patch.object(queue.urllib.request, "urlopen", return_value=response):
                queue.main()
            self.assertEqual(output.read_text(), "pending=true\njob_id=12\n")

    def test_empty_queue(self):
        self.assertEqual(queue.find_pending(lambda _: []), "")

    def test_api_failure_does_not_appear_as_an_empty_queue(self):
        def fail(_):
            raise RuntimeError("API unavailable")
        with self.assertRaises(RuntimeError):
            queue.find_pending(fail)


if __name__ == "__main__":
    unittest.main()
