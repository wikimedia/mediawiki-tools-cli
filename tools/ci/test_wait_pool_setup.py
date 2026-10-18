"""Piped credentials must be read before SSH gets a chance to consume stdin."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("setup", Path(__file__).with_name("configure-wait-pool.py"))
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)


class SetupTests(unittest.TestCase):
    def test_preflight_cannot_consume_piped_read_token(self):
        source, output, installs = io.StringIO("test-only-read-token"), io.StringIO(), []
        def check_output(command, **kwargs):
            self.assertEqual(source.read(), "")  # Already captured by the helper.
            self.assertEqual(kwargs["stdin"], subprocess.DEVNULL)
            return "/tmp/mwcli-wait.test\n" if "mktemp" in command[-1] else '{"id":1551}'
        def run(command, **kwargs):
            self.assertNotIn("test-only-read-token", " ".join(command))
            if "python3" in command:
                installs.append(json.loads(kwargs["input"]))
            return subprocess.CompletedProcess(command, 0, "installed\n", "")
        with patch("sys.argv", ["setup", "addshore@runner.example", "--bot-id", "7", "--read-token-stdin"]), patch(
            "sys.stdin", source
        ), patch.object(setup.subprocess, "check_output", side_effect=check_output), patch.object(
            setup.subprocess, "run", side_effect=run
        ), contextlib.redirect_stdout(output):
            setup.main()
        self.assertEqual(installs, [{"id": 1551, "bot_id": 7,
                                     "repository": "wikimedia/mediawiki-tools-cli",
                                     "read_token": "test-only-read-token"}])
        self.assertNotIn("test-only-read-token", output.getvalue())


if __name__ == "__main__":
    unittest.main()
