#!/usr/bin/env python3
"""Create or update a Cloud VPS waiting pool from the local checkout."""
import argparse
import getpass
import json
from pathlib import Path
import re
import shlex
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("host", help="SSH destination, e.g. addshore@runner.example")
    parser.add_argument("--jump", default="addshore@primary.bastion.wmcloud.org")
    parser.add_argument("--mw", default="mw", help="path to the mw CLI")
    parser.add_argument("--bot-id", type=int)
    parser.add_argument("--repository", choices=["wikimedia/mediawiki-tools-cli", "addshore/testing"],
                        default="wikimedia/mediawiki-tools-cli")
    parser.add_argument("--read-token", action="store_true", help="prompt securely for an existing read_api token")
    parser.add_argument("--read-token-stdin", action="store_true", help=argparse.SUPPRESS)
    args = parser.parse_args()
    for value in (args.host, args.jump):
        if not re.fullmatch(r"[A-Za-z0-9_.@-]+", value) or value.startswith("-"):
            parser.error("expected an SSH destination")
    # Read piped credentials before spawning SSH: otherwise ssh can drain stdin.
    read_token = None
    if args.read_token or args.read_token_stdin:
        if args.bot_id is None:
            parser.error("--bot-id is required when installing the read token")
        read_token = (sys.stdin.read().strip() if args.read_token_stdin else
                      getpass.getpass("Existing GitLab read_api token: "))
        if not read_token:
            parser.error("read token is empty")
    ssh = ["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-J", args.jump, args.host]
    probe = "sudo -n python3 -c " + shlex.quote(
        "import json,pathlib,tomllib; p=pathlib.Path('/etc/gitlab-runner-wait/config.toml'); "
        "print(json.dumps({'id':tomllib.loads(p.read_text())['runners'][0]['id']} if p.exists() else {}))")
    existing = json.loads(subprocess.check_output(ssh + [probe], text=True, stdin=subprocess.DEVNULL))
    data = dict(existing)
    if args.bot_id is not None:
        if args.bot_id <= 0:
            parser.error("bot ID must be positive")
        data.update(bot_id=args.bot_id, repository=args.repository)
    if read_token is not None:
        data["read_token"] = read_token
    folder = subprocess.check_output(ssh + ["mktemp -d /tmp/mwcli-wait.XXXXXX"], text=True, stdin=subprocess.DEVNULL).strip()
    if not re.fullmatch(r"/tmp/mwcli-wait\.[A-Za-z0-9]+", folder):
        raise ValueError("unexpected remote temporary directory")
    created = None
    try:
        for name in ("wait-runner.py", "wait-status.py", "wait-github.py", "github_status.py"):
            source = Path(__file__).with_name(name).read_bytes()
            subprocess.run(ssh + [f"cat > {folder}/{name}"], input=source, check=True)
        if "id" not in data:
            name = args.host.split("@")[-1].split(".")[0] + "-wait"
            payload = {"runner_type": "project_type", "project_id": 16, "description": name,
                       "tag_list": ["mwcli-wait", name], "locked": True, "run_untagged": False,
                       "access_level": "not_protected", "maximum_timeout": 3600}
            result = subprocess.run([args.mw, "gl", "api", "user/runners", "--method", "POST",
                                     "-H", "Content-Type:application/json", "--input", "-"],
                                    input=json.dumps(payload), text=True, capture_output=True)
            if result.returncode:
                raise RuntimeError("runner registration failed; check mw gl authentication")
            created = json.loads(result.stdout)
            data.update(id=created["id"], token=created["token"])
        result = subprocess.run(ssh + ["sudo", "-n", "python3", folder + "/wait-runner.py"],
                                input=json.dumps(data), text=True, capture_output=True)
        if result.returncode:
            # Installer errors contain no credential values; don't print registration responses.
            raise RuntimeError("remote installation failed: " + result.stderr)
        print(result.stdout.strip())
        print("Existing services are unchanged. Updated pool configuration takes effect on its next job poll.")
    except Exception:
        if created:
            print(f"New registration {created['id']} exists; inspect it before retrying setup.", file=sys.stderr)
        raise
    finally:
        subprocess.run(ssh + ["rm", "-rf", folder], check=True, stdin=subprocess.DEVNULL)


if __name__ == "__main__":
    main()
