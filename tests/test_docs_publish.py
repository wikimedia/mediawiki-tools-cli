"""Exercise selected-ref publishing without contacting a wiki or running Go."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


MODULE = Path(__file__).resolve().parents[1] / "tools" / "docs-publish.py"
SPEC = importlib.util.spec_from_file_location("docs_publish", MODULE)
DOCS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DOCS)

PUBLISHER = '''import json, os, pathlib, sys
root = pathlib.Path.cwd()
generated = json.loads((root / "_docs" / "pages.json").read_text())
(root / "_docs" / "publication.json").write_text(json.dumps({
    "generated": generated, "version": os.environ["DOCS_VERSION"],
    "summary": os.environ.get("DOCS_SUMMARY"), "mode": sys.argv[2],
    "has_password": bool(os.environ.get("DOCS_UPDATE_PASS")),
    "publisher_source": PUBLISHER_SOURCE,
}))
sys.exit(int(os.environ.get("FIXTURE_PUBLISH_EXIT", "0")))
'''


class DocumentationPublishTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.root = self.directory / "repo"
        self.root.mkdir()
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.name", "Fixture")
        self.git("config", "user.email", "fixture@example.invalid")
        (self.root / "source.txt").write_text("release source")
        self.git("add", "source.txt")
        self.git("commit", "-qm", "release")
        self.git("tag", "v0.34.0")
        (self.root / "source.txt").write_text("current source")
        self.git("commit", "-qam", "current controller")
        remote = self.directory / "remote.git"
        subprocess.run(["git", "init", "--bare", "-q", str(remote)], check=True)
        self.git("remote", "add", "origin", str(remote))
        self.git("push", "-q", "origin", "main", "--tags")
        binary = self.directory / "bin"
        binary.mkdir()
        fake_go = f'''import pathlib, sys
source = pathlib.Path("source.txt").read_text()
publisher = pathlib.Path(sys.argv[sys.argv.index("-o") + 1])
publisher.write_text({('#!' + sys.executable + chr(10))!r} + "PUBLISHER_SOURCE = " + repr(source) + "\\n" + {PUBLISHER!r})
publisher.chmod(0o700)
'''
        fake_mise = '''import json, os, pathlib, sys
if sys.argv[1] == "exec":
    root = pathlib.Path.cwd()
    (root / "_docs").mkdir()
    (root / "_docs" / "pages.json").write_text(json.dumps({
        "source": (root / "source.txt").read_text(),
        "has_password": bool(os.environ.get("DOCS_UPDATE_PASS")),
    }))
'''
        for name, body in (("go", fake_go), ("mise", fake_mise)):
            path = binary / name
            path.write_text(f"#!{sys.executable}\n" + body)
            path.chmod(0o700)
        self.env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ["PATH"], DOCS_REF="v0.34.0", DOCS_UPDATE_PASS="fixture-password", DOCS_SUMMARY="Refresh release docs")

    def git(self, *args):
        return subprocess.check_output(["git", *args], cwd=self.root, text=True).strip()

    def test_release_uses_release_source_and_current_publisher(self):
        DOCS.publish(self.root, self.env)
        result = json.loads((self.root / "_docs" / "publication.json").read_text())
        self.assertEqual(result["generated"]["source"], "release source")
        self.assertEqual(result["publisher_source"], "current source")
        self.assertEqual(result["version"], "v0.34.0")
        self.assertEqual(result["summary"], "Refresh release docs")
        self.assertFalse(result["generated"]["has_password"])
        self.assertTrue(result["has_password"])
        self.assertEqual(self.git("branch", "--show-current"), "main")
        self.assertEqual(self.git("worktree", "list", "--porcelain").count("worktree "), 1)

    def test_commit_selection_can_have_a_separate_release_label(self):
        commit = self.git("rev-parse", "HEAD")
        DOCS.publish(self.root, dict(self.env, DOCS_REF=commit, DOCS_VERSION="v0.34.0 corrections", DOCS_MODE="check"))
        result = json.loads((self.root / "_docs" / "publication.json").read_text())
        self.assertEqual(result["generated"]["source"], "current source")
        self.assertEqual(result["version"], "v0.34.0 corrections")
        self.assertEqual(result["mode"], "check")

    def test_failure_keeps_reports_and_cleans_up_the_worktree(self):
        with self.assertRaises(subprocess.CalledProcessError):
            DOCS.publish(self.root, dict(self.env, FIXTURE_PUBLISH_EXIT="1"))
        self.assertTrue((self.root / "_docs" / "publication.json").exists())
        self.assertTrue((self.root / "_docs" / "source.json").exists())
        self.assertEqual(self.git("worktree", "list", "--porcelain").count("worktree "), 1)

    def test_invalid_requests_fail_before_building_or_fetching(self):
        for ref, mode in (("", "publish"), ("--upload-pack=bad", "publish"), ("main:refs/heads/main", "publish"), ("main other", "publish"), ("main", "bad-mode")):
            with self.subTest(ref=ref, mode=mode), self.assertRaises(ValueError):
                DOCS.publish(self.root, dict(self.env, DOCS_REF=ref, DOCS_MODE=mode))


if __name__ == "__main__":
    unittest.main()
