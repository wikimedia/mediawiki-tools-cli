#!/usr/bin/env python3
"""Exercise native executable replacement and config access without Docker."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
from datetime import datetime, timezone


def digest(path):
    result = hashlib.sha256()
    with path.open("rb") as binary:
        for chunk in iter(lambda: binary.read(1024 * 1024), b""):
            result.update(chunk)
    return result.hexdigest()


def main():
    filename = "mw.exe" if os.name == "nt" else "mw"
    original = Path("bin", filename).resolve()
    with tempfile.TemporaryDirectory(prefix="mwcli-update-") as temporary:
        root = Path(temporary)
        installed = root / "installed cli" / filename
        replacement = root / "update artifact" / filename
        installed.parent.mkdir()
        replacement.parent.mkdir()
        shutil.copy2(original, installed)
        subprocess.run([
            "go", "build", "-ldflags=-X main.GitCommit=platform-update-fixture",
            "-o", str(replacement), ".",
        ], check=True)
        expected = digest(replacement)
        if digest(installed) == expected:
            raise RuntimeError("replacement must differ from the installed binary")

        config_home = root / "config home"
        config_dir = config_home / "mwcli"
        config_dir.mkdir(parents=True)
        # Suppress the unrelated startup release check and telemetry. The CLI
        # still loads and writes this real, isolated configuration normally.
        config_file = config_dir / "config.json"
        config_file.write_text(json.dumps({
            "telemetry": "no",
            "timer_last_update_checked": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        }))
        config_file.chmod(0o600)
        env = os.environ.copy()
        for key in ("GITLAB_CI", "CI_PROJECT_DIR"):
            env.pop(key, None)
        env.update({
            "MWCLI_CONTEXT_TEST": "1", "XDG_CONFIG_HOME": str(config_home),
            "APPDATA": str(config_home), "LOCALAPPDATA": str(config_home),
        })

        def mw(*args):
            result = subprocess.run(
                [str(installed), "--no-interaction", *args], env=env,
                capture_output=True, text=True, timeout=60,
            )
            if result.returncode:
                raise RuntimeError(f"mw {args} failed:\n{result.stdout}{result.stderr}")
            return result.stdout.strip()

        mw("config", "set", "gerrit.username", "before-update")
        mw("update", "--version", str(replacement))
        # Windows stages replacement until the original process releases its
        # executable; Unix completes it before the update command returns.
        deadline = time.monotonic() + 30
        while True:
            try:
                replaced = digest(installed) == expected
            except OSError:
                replaced = False
            if replaced:
                break
            if time.monotonic() >= deadline:
                raise RuntimeError("self-update did not install the replacement binary")
            time.sleep(0.2)
        mw("config", "show")
        if mw("config", "get", "gerrit.username") != "before-update":
            raise RuntimeError("self-update did not preserve existing config")
        mw("config", "set", "gerrit.username", "after-update")
        if mw("config", "get", "gerrit.username") != "after-update":
            raise RuntimeError("updated binary cannot write user config")
        print("PASS: native update replaced the binary and preserved readable/writable config")


if __name__ == "__main__":
    main()
