#!/usr/bin/env python3
"""Exercise native config persistence, paths with spaces and embedded dev files.

Run with MWCLI_TEST_BINARY pointing to the native binary. No Docker daemon,
container images or MediaWiki checkout are needed for this test.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile


def main():
    binary = Path(os.environ["MWCLI_TEST_BINARY"]).resolve()
    source = Path(__file__).resolve().parents[1]
    with tempfile.TemporaryDirectory(prefix="mwcli platform ") as temporary:
        root = Path(temporary).resolve()
        home, config = root / "home", root / "config"
        home.mkdir()
        config.mkdir()
        env = os.environ.copy()
        env.pop("GITLAB_CI", None)
        env.pop("CI_PROJECT_DIR", None)
        env.update({"HOME": str(home), "USERPROFILE": str(home),
                    "XDG_CONFIG_HOME": str(config), "APPDATA": str(config),
                    "LOCALAPPDATA": str(config), "MWCLI_MEDIAWIKI_ENV_COMMAND": "1"})

        def mw(*args):
            print("+ mw", *args, flush=True)
            result = subprocess.run([str(binary), "--no-interaction", *args],
                                    cwd=source, env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=True)
            print(result.stdout, end="", flush=True)
            return result.stdout.strip()

        mw("version")
        mw("config", "set", "telemetry", "platform-filesystem")
        if mw("config", "get", "telemetry") != "platform-filesystem":
            raise AssertionError("config get did not return the persisted value")
        path = Path(mw("config", "where")).resolve()
        if not path.is_relative_to(root) or not path.is_file():
            raise AssertionError(f"config was not created in the isolated home: {path}")
        if json.loads(path.read_text())["telemetry"] != "platform-filesystem":
            raise AssertionError("on-disk config does not contain the persisted value")
        # A second write must work too; this does not exercise sudo ownership.
        mw("config", "set", "telemetry", "no")
        if mw("config", "get", "telemetry") != "no":
            raise AssertionError("config could not be updated")

        prefix = ("docker", "--context", "ci-platform-filesystem")
        mw(*prefix, "env", "set", "MEDIAWIKI_VOLUMES_CODE", str(source))
        if mw(*prefix, "env", "get", "MEDIAWIKI_VOLUMES_CODE") != str(source):
            raise AssertionError("development environment path did not round-trip")
        directory = Path(mw(*prefix, "where")).resolve()
        if not directory.is_relative_to(root):
            raise AssertionError("development files escaped the isolated home")
        for entry in (source / "mount/dev/files.txt").read_text().splitlines():
            if entry and not (directory / entry.removeprefix("./")).is_file():
                raise AssertionError(f"embedded development file is missing: {entry}")
        expected = source / "mount/dev/compose/base/compose.yml"
        actual = directory / "compose/base/compose.yml"
        if actual.read_bytes() != expected.read_bytes():
            raise AssertionError("embedded Compose file differs from the tested source")
    print("PASS: native config persistence and embedded development files")


if __name__ == "__main__":
    main()
