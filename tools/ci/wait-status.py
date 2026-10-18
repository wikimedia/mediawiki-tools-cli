#!/usr/bin/env python3
"""Read GitLab results on behalf of authenticated waiting jobs, over a local socket.

The project read token stays on the host. This service accepts no API paths,
project IDs, commit SHAs or result receipts from its clients.
"""
import copy
import http.server
import json
import os
from pathlib import Path
import socketserver
import threading
import urllib.error
import urllib.request

from github_status import (API, CONTEXTS, GATE_JOB, PROJECT, REQUEST_JOB, GitLab,
                           SafeRedirect, commit_sha, pipeline_jobs, statuses, verify_results)


def bundle_head(stream):
    header = stream.readline(128)
    if header not in {b"# v2 git bundle\n", b"# v3 git bundle\n"}:
        raise ValueError("invalid merge bundle")
    size = len(header)
    for _ in range(128):
        line = stream.readline(1024)
        size += len(line)
        if size > 65536 or not line:
            break
        if line.endswith(b" HEAD\n"):
            return commit_sha(line.split()[0].decode("ascii"))
        if line == b"\n":
            break
    raise ValueError("merge bundle has no HEAD")


def check(api, job, config):
    if (job.get("name") != GATE_JOB or job.get("status") != "running"
            or job.get("pipeline", {}).get("project_id") != PROJECT):
        raise PermissionError("expected an active upstream integration waiting job")
    pipeline_id = job["pipeline"]["id"]
    pipeline = api.get(f"/projects/{PROJECT}/pipelines/{pipeline_id}")
    jobs = pipeline_jobs(api, pipeline_id)
    if (pipeline["status"] in {"canceled", "canceling"}
            or jobs.get(GATE_JOB, {}).get("id") != job["id"]):
        raise PermissionError("waiting job is canceled or superseded")
    request = jobs.get(REQUEST_JOB, {})
    if request.get("status") != "success":
        return {"state": "pending", "message": "Waiting for the integration request"}
    sha = commit_sha(pipeline["sha"])
    tested_sha = sha
    merge = jobs.get("prepare-merge")
    if merge:
        if merge["status"] != "success":
            raise ValueError("merged source preparation has not passed")
        url = API + f"/projects/{PROJECT}/jobs/{merge['id']}/artifacts/.ci-merge.bundle"
        req = urllib.request.Request(url, headers={"PRIVATE-TOKEN": api.token})
        with urllib.request.build_opener(SafeRedirect()).open(req, timeout=30) as stream:
            tested_sha = bundle_head(stream)
    records = statuses(api, sha, pipeline_id)
    # Validate the identity even for failed results before treating them as terminal.
    latest = {}
    for record in records:
        if record["name"] in CONTEXTS and record["id"] > latest.get(record["name"], {}).get("id", 0):
            latest[record["name"]] = record
    if len(latest) != len(CONTEXTS):
        return {"state": "pending", "message": "Waiting for GitHub results"}
    normalized = copy.deepcopy(list(latest.values()))
    terminal = {"success", "failed", "canceled"}
    if any(record["status"] not in terminal for record in normalized):
        return {"state": "pending", "message": "GitHub integration tests are running"}
    for record in normalized:
        record["status"] = "success"
    try:
        batch = verify_results(normalized, bot_id=config["bot_id"], request_id=request["id"],
                               tested_sha=tested_sha, repository=config["repository"])
    except ValueError:
        # Partial callbacks/reruns can temporarily belong to different attempts.
        return {"state": "pending", "message": "Waiting for a complete authenticated result batch"}
    failed = [name for name, record in latest.items() if record["status"] != "success"]
    return {"state": "failed" if failed else "success",
            "message": ("Failed: " + ", ".join(failed)) if failed else "All four GitHub tests passed",
            "url": f"https://github.com/{config['repository']}/actions/runs/{batch[1]}"}


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass  # Never log job tokens or request headers.

    def do_GET(self):
        code, result = 503, {"state": "pending", "message": "Result service temporarily unavailable"}
        try:
            if self.path != "/status":
                raise PermissionError("unknown endpoint")
            token = self.headers.get("JOB-TOKEN", "")
            if not token or len(token) > 4096:
                raise PermissionError("missing job identity")
            request = urllib.request.Request(API + "/job", headers={"JOB-TOKEN": token})
            with urllib.request.urlopen(request, timeout=20) as response:
                job = json.load(response)
            result = check(self.server.api, job, self.server.config)
            code = 200
        except PermissionError:
            code, result = 403, {"state": "failed", "message": "Waiting job identity was rejected"}
        except urllib.error.HTTPError as error:
            if error.code in {401, 403}:
                code, result = 403, {"state": "failed", "message": "GitLab rejected authentication"}
        except (ValueError, KeyError, OSError, TimeoutError):
            pass  # Fail closed; the client has a bounded overall deadline.
        body = json.dumps(result).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class Server(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True
    slots = threading.BoundedSemaphore(8)

    def process_request_thread(self, request, client_address):
        with self.slots:
            super().process_request_thread(request, client_address)


if __name__ == "__main__":
    config = json.loads(Path("/etc/mwcli-wait-status.json").read_text())
    credential = Path(os.environ["CREDENTIALS_DIRECTORY"]) / "gitlab-read-token"
    socket = Path("/run/mwcli-wait/status.sock")
    socket.unlink(missing_ok=True)
    with Server(str(socket), Handler) as server:
        server.config = config
        server.api = GitLab(credential.read_text().strip())
        os.chmod(socket, 0o666)  # Each caller must authenticate with its own CI_JOB_TOKEN.
        server.serve_forever()
