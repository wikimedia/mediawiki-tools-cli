"""Check release failure diagnostics without building or filling a disk."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("release-build.sh")


class ReleaseBuildTest(unittest.TestCase):
    def test_success_and_failure_preserve_build_arguments_and_status(self):
        for status in (0, 2, 42):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as temporary:
                directory = Path(temporary)
                binary = directory / "bin"
                binary.mkdir()
                arguments = directory / "arguments.json"
                fake_make = binary / "make"
                fake_make.write_text(
                    f"#!{sys.executable}\n"
                    "import json, os, pathlib, sys\n"
                    "pathlib.Path(os.environ['FIXTURE_ARGUMENTS']).write_text(json.dumps(sys.argv[1:]))\n"
                    "sys.exit(int(os.environ['FIXTURE_STATUS']))\n"
                )
                fake_make.chmod(0o700)
                # Diagnostics may themselves fail on a broken runner; that
                # must not replace the original build's exit status.
                for name in ("df", "du"):
                    command = binary / name
                    command.write_text("#!/bin/sh\nexit 1\n")
                    command.chmod(0o700)
                environment = dict(
                    os.environ,
                    PATH=str(binary) + os.pathsep + os.environ["PATH"],
                    FIXTURE_ARGUMENTS=str(arguments), FIXTURE_STATUS=str(status),
                    GOCACHE=str(directory), GOMODCACHE=str(directory),
                )
                result = subprocess.run(
                    ["bash", str(SCRIPT), "VERSION=feature/release", "RELEASE_PARALLEL=4"],
                    cwd=directory, env=environment, capture_output=True, text=True,
                )
                self.assertEqual(result.returncode, status)
                self.assertEqual(json.loads(arguments.read_text()), ["release", "VERSION=feature/release", "RELEASE_PARALLEL=4"])
                self.assertIn("disk usage (before)", result.stderr)
                self.assertIn("inode usage (before)", result.stderr)
                self.assertEqual("disk usage (failed)" in result.stderr, status != 0)
                if status:
                    self.assertIn(f"Release build failed (exit {status})", result.stderr)


if __name__ == "__main__":
    unittest.main()
