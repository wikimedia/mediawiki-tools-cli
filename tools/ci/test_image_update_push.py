"""Exercise image-update publication against disposable local Git remotes."""

import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "image-update/push.sh"
BOT = "project16_bot@noreply.gitlab.wikimedia.org"
DESCRIPTION = "Bump mariadb from 11.4 to 11.8"
BRANCH = DESCRIPTION.replace(" ", "_")


class PushTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.remote = self.root / "remote.git"
        self.work = self.root / "work"
        self.work.mkdir()
        self.env = {
            **os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
            "DESCRIPTION": DESCRIPTION, "CI_PROJECT_URL": "https://example.test/project",
        }
        self.git("init", "--bare", str(self.remote))
        self.git("init", "-b", "main")
        self.git("config", "user.email", BOT)
        self.git("config", "user.name", "image-update-ci")
        self.git("config", "commit.gpgsign", "false")
        self.git("--git-dir", str(self.remote), "config", "receive.advertisePushOptions", "true")
        self.git("remote", "add", "origin", str(self.remote))
        (self.work / "image").write_text("old\n")
        self.git("add", "image")
        self.git("commit", "-m", "initial")
        self.git("push", "origin", "main")

    def git(self, *args, input=None):
        return subprocess.run(
            ["git", *args], cwd=self.work, env=self.env, input=input,
            text=True, capture_output=True, check=True,
        ).stdout.strip()

    def remote_tip(self):
        return self.git("ls-remote", "--heads", "origin", "refs/heads/" + BRANCH).split("\t")[0]

    def existing_update(self, *, author=BOT, committer=BOT):
        self.git("checkout", "-b", BRANCH)
        (self.work / "image").write_text("new\n")
        self.git("-c", "user.email=" + committer, "commit", "-am", DESCRIPTION,
                 "--author=Contributor <" + author + ">")
        self.git("push", "origin", BRANCH)
        tip = self.remote_tip()
        self.git("checkout", "main")
        (self.work / "unrelated").write_text("new main commit\n")
        self.git("add", "unrelated")
        self.git("commit", "-m", "advance main")
        self.git("push", "origin", "main")
        return tip

    def publish(self, *, changes=True):
        if changes:
            (self.work / "image").write_text("new\n")
        return subprocess.run(
            ["bash", str(SCRIPT)], cwd=self.work, env=self.env,
            text=True, capture_output=True,
        )

    def test_creates_new_branch(self):
        base = self.git("rev-parse", "HEAD")
        result = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.remote_tip())
        self.assertEqual(self.git("rev-parse", "HEAD^"), base)

    def test_refreshes_bot_branch_on_current_main(self):
        old = self.existing_update()
        base = self.git("rev-parse", "HEAD")
        result = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotEqual(self.remote_tip(), old)
        self.assertEqual(self.git("rev-parse", "HEAD^"), base)
        self.assertEqual(self.git("rev-list", "--count", "HEAD", "--not", base), "1")
        self.assertEqual(self.git("show", "HEAD:unrelated"), "new main commit")

    def test_preserves_human_authored_commit(self):
        self.existing_update()
        self.git("checkout", BRANCH)
        (self.work / "image").write_text("human changes\n")
        self.git("commit", "-am", "adjust update", "--author=Human <human@example.test>")
        self.git("push", "origin", BRANCH)
        old = self.remote_tip()
        self.git("checkout", "main")
        result = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Skipped", result.stdout)
        self.assertIn("source_branch=" + BRANCH, result.stdout)
        self.assertEqual(self.remote_tip(), old)

    def test_preserves_human_committed_amendment(self):
        old = self.existing_update(committer="human@example.test")
        result = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Skipped", result.stdout)
        self.assertEqual(self.remote_tip(), old)

    def test_no_changes_is_successful_skip(self):
        # The generated no-updates matrix has no DESCRIPTION.
        del self.env["DESCRIPTION"]
        result = self.publish(changes=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("no image changes", result.stdout)
        self.assertEqual(self.remote_tip(), "")

    def test_concurrent_changes_are_preserved(self):
        self.existing_update()
        self.check_concurrent_change()

    def test_concurrent_branch_creation_is_preserved(self):
        self.check_concurrent_change()

    def check_concurrent_change(self):
        tree = self.git("rev-parse", "HEAD^{tree}")
        newer = self.git("--git-dir", str(self.remote), "-c", "user.name=Human",
                         "-c", "user.email=human@example.test", "commit-tree", tree,
                         "-p", self.remote_tip() or self.git("rev-parse", "HEAD"),
                         input="concurrent change\n")
        hook = self.work / ".git/hooks/pre-push"
        hook.write_text("#!/bin/sh\ngit --git-dir " + shlex.quote(str(self.remote))
                        + " update-ref " + shlex.quote("refs/heads/" + BRANCH)
                        + " " + newer + "\n")
        hook.chmod(0o700)
        result = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("changed during this run", result.stdout)
        self.assertEqual(self.remote_tip(), newer)

    def test_other_push_errors_still_fail(self):
        hook = self.remote / "hooks/pre-receive"
        hook.write_text("#!/bin/sh\necho 'simulated server failure' >&2\nexit 1\n")
        hook.chmod(0o700)
        result = self.publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("simulated server failure", result.stderr)
        self.assertNotIn("Skipped", result.stdout)


if __name__ == "__main__":
    unittest.main()
