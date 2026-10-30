#!/usr/bin/env python3
"""Generate a selected ref with its CLI code, then publish with current tooling."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def run(*args, cwd, env=None):
    subprocess.run(args, cwd=cwd, env=env, check=True)


def publish(root, environ):
    ref = environ.get("DOCS_REF", "").strip()
    if not ref or ref.startswith(("-", "+")) or any(c in ref for c in ":*?[]^~\\") or any(c.isspace() or ord(c) < 32 for c in ref):
        raise ValueError("Set DOCS_REF to the branch, tag or commit whose documentation should be generated")
    mode = environ.get("DOCS_MODE", "publish")
    if mode not in ("publish", "check"):
        raise ValueError("DOCS_MODE must be publish or check")
    label = environ.get("DOCS_VERSION", "").strip() or ref
    with tempfile.TemporaryDirectory(prefix="mwcli-docs-") as temporary:
        temporary = Path(temporary)
        publisher = temporary / "docs-publisher"
        source = temporary / "source"
        env = dict(environ, DOCS_VERSION=label, GOFLAGS="-mod=readonly")
        build_env = {k: v for k, v in env.items() if k not in ("DOCS_UPDATE_USER", "DOCS_UPDATE_PASS", "user", "password")}
        # CLI initialization can create runtime config under CI_PROJECT_DIR;
        # keep that outside the selected source checkout and its version state.
        runtime = temporary / "runtime"
        runtime.mkdir()
        build_env["CI_PROJECT_DIR"] = str(runtime)
        env["CI_PROJECT_DIR"] = str(runtime)
        run("go", "build", "-o", str(publisher), "./tools/docs-gen", cwd=root, env=build_env)
        run("git", "fetch", "--no-tags", "--", "origin", ref, cwd=root, env=build_env)
        commit = subprocess.check_output(["git", "rev-parse", "--verify", "FETCH_HEAD^{commit}"], cwd=root, env=build_env, text=True).strip()
        run("git", "worktree", "add", "--detach", str(source), commit, cwd=root, env=build_env)
        try:
            print(f"Documentation source: {ref} ({commit}); version label: {label}; mode: {mode}", flush=True)
            run("mise", "trust", "mise.toml", cwd=source, env=build_env)
            run("mise", "exec", "--", "go", "run", "./tools/docs-gen", cwd=source, env=build_env)
            (source / "_docs" / "source.json").write_text(json.dumps({"ref": ref, "commit": commit, "version": label, "summary": env.get("DOCS_SUMMARY", ""), "mode": mode}, indent=2) + "\n")
            run(str(publisher), "-mode", mode, cwd=source, env=env)
        finally:
            # Preserve generation and drift reports even after partial failure.
            if (source / "_docs").exists():
                shutil.copytree(source / "_docs", root / "_docs", dirs_exist_ok=True)
            run("git", "worktree", "remove", "--force", str(source), cwd=root, env=build_env)


if __name__ == "__main__":
    publish(Path(__file__).resolve().parents[1], os.environ)
