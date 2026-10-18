#!/usr/bin/env python3
"""Build and run a GitLab-selected test on a runner without callback credentials."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from github_status import TESTS


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["checkout", "build", "test"])
    args = parser.parse_args()
    inputs = Path("inputs").resolve()
    source = Path("source").resolve()
    plan = json.loads((inputs / "plan.json").read_text())
    if args.command == "checkout":
        source.mkdir()
        subprocess.run(["git", "init", str(source)], check=True)
        subprocess.run(["git", "-C", str(source), "fetch", str(inputs / "source.bundle"), "HEAD"], check=True)
        subprocess.run(["git", "-C", str(source), "checkout", "--detach", "FETCH_HEAD"], check=True)
    actual = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    if actual != plan["tested_sha"]:
        raise ValueError("source snapshot does not match the prepared commit")
    binary = source / "bin" / ("mw.exe" if sys.platform == "win32" else "mw")
    if args.command == "build":
        binary.parent.mkdir(exist_ok=True)
        subprocess.run(["go", "build", "-o", str(binary), "."], cwd=source, check=True)
    elif args.command == "test":
        test = plan["test"]
        if not re.fullmatch(r"[a-z][a-z0-9-]{0,63}", test):
            raise ValueError("expected a test name, without a path or arguments")
        candidates = [source / "tests" / TESTS[test]] if test in TESTS else [
            source / "tests" / f"test-{test}.{extension}" for extension in ("py", "sh")]
        available = [script for script in candidates if script.is_file()]
        if len(available) != 1:
            raise ValueError("expected exactly one Python or shell test script in the tested source")
        script = available[0]
        interpreter = sys.executable if script.suffix == ".py" else "bash"
        env = {**os.environ, "MWCLI_TEST_BINARY": str(binary)}
        subprocess.run([interpreter, str(script)], cwd=source, env=env, check=True)


if __name__ == "__main__":
    main()
