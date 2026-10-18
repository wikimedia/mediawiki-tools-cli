"""The native runner must execute tests from the verified source bundle."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

RUNNER = Path(__file__).with_name("run-platform-integration.py").resolve()


class SourceTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="mwcli runner ")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        repo = self.root / "repository"
        repo.mkdir()
        self.work = self.root / "runner"
        self.work.mkdir()
        inputs = self.work / "inputs"
        inputs.mkdir()
        def git(*args):
            return subprocess.check_output(["git", *args], cwd=repo, text=True,
                                           stderr=subprocess.DEVNULL).strip()
        git("init")
        git("config", "user.email", "test@example.invalid")
        git("config", "user.name", "CI test")
        (repo / "tests").mkdir()
        (repo / "tests/test-selected.py").write_text(
            "import os\nfrom pathlib import Path\n"
            "assert Path(os.environ['MWCLI_TEST_BINARY']).parent.name == 'bin'\n"
            "Path('executed-source-test').write_text('from GitLab bundle')\n"
        )
        (repo / "tests/test-docker-general.sh").write_text(
            "#!/usr/bin/env bash\nset -eu\nprintf 'existing shell test' > executed-shell-test\n")
        git("add", ".")
        git("commit", "-m", "Test source")
        sha = git("rev-parse", "HEAD")
        git("bundle", "create", str(inputs / "source.bundle"), "HEAD")
        self.plan = {"tested_sha": sha, "test": "selected"}
        self.save_plan()

    def save_plan(self):
        (self.work / "inputs/plan.json").write_text(json.dumps(self.plan))

    def run_step(self, step):
        return subprocess.run([sys.executable, str(RUNNER), step], cwd=self.work,
                              text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

    def test_runs_the_selected_script_from_the_bundle(self):
        result = self.run_step("checkout")
        self.assertEqual(result.returncode, 0, result.stdout)
        result = self.run_step("test")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual((self.work / "source/executed-source-test").read_text(), "from GitLab bundle")

    def test_wrong_snapshot_cannot_execute_tests(self):
        self.assertEqual(self.run_step("checkout").returncode, 0)
        self.plan["tested_sha"] = "a" * 40
        self.save_plan()
        result = self.run_step("test")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("source snapshot does not match", result.stdout)
        self.assertFalse((self.work / "source/executed-source-test").exists())

    def test_existing_docker_test_name_uses_its_shell_script(self):
        self.plan["test"] = "docker-general"
        self.save_plan()
        self.assertEqual(self.run_step("checkout").returncode, 0)
        result = self.run_step("test")
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual((self.work / "source/executed-shell-test").read_text(), "existing shell test")

    def test_selector_cannot_escape_the_tests_directory(self):
        self.assertEqual(self.run_step("checkout").returncode, 0)
        self.plan["test"] = "../selected"
        self.save_plan()
        result = self.run_step("test")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("expected a test name", result.stdout)


if __name__ == "__main__":
    unittest.main()
