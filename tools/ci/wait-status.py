#!/usr/bin/env python3
"""Dispatch GitHub tests and read results for authenticated jobs over a local socket.

The project read and GitHub dispatch tokens stay on the host. This service accepts no API paths,
project IDs, commit SHAs or result receipts from its clients.
"""
import http.server
import json
import os
import re
from pathlib import Path
import socketserver
import threading
import urllib.error
import urllib.request

from github_status import (API, SUMMARY_CONTEXT, GATE_JOB, PROJECT, REQUEST_JOB, GitLab,
                           SafeRedirect, REPOSITORY, positive_id, commit_sha, pipeline_jobs, statuses, verify_summary, verify_status, gate_test, gate_jobs)


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
    name = job.get("name", "")
    selected = gate_test(name)
    if ((name != GATE_JOB and selected is None) or job.get("status") != "running"
            or job.get("pipeline", {}).get("project_id") != PROJECT):
        raise PermissionError("expected an active upstream integration waiting job")
    pipeline_id = job["pipeline"]["id"]
    pipeline = api.get(f"/projects/{PROJECT}/pipelines/{pipeline_id}")
    jobs = pipeline_jobs(api, pipeline_id)
    if (pipeline["status"] in {"canceled", "canceling"}
            or jobs.get(name, {}).get("id") != job["id"]):
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
    all_records = statuses(api, sha, pipeline_id)
    records = [record for record in all_records if record.get("name") == SUMMARY_CONTEXT]
    if not records:
        return {"state": "pending", "message": "Waiting for the GitHub workflow result"}
    summary = max(records, key=lambda record: record["id"])
    try:
        batch = verify_summary(summary, bot_id=config["bot_id"], request_id=request["id"],
                               tested_sha=tested_sha, repository=config["repository"])
    except ValueError:
        return {"state": "pending", "message": "Waiting for an authenticated workflow result"}
    state = summary.get("status")
    if state not in {"success", "failed", "canceled"}:
        return {"state": "pending", "message": "GitHub integration tests are running"}
    if state == "success" and selected is not None:
        context = "github/" + selected["name"]
        tests = [record for record in all_records if record.get("name") == context]
        if not tests:
            return {"state": "pending", "message": "Waiting for this test's authenticated result"}
        result = max(tests, key=lambda record: record["id"])
        try:
            identity = verify_status(result, context, bot_id=config["bot_id"], request_id=request["id"],
                                     tested_sha=tested_sha, repository=config["repository"])
        except ValueError:
            return {"state": "pending", "message": "Waiting for this test's authenticated result"}
        if identity != batch or result.get("status") != "success":
            return {"state": "pending", "message": "Waiting for this test's matching workflow result"}
    return {"state": "success" if state == "success" else "failed",
            "message": "All GitHub integration tests passed" if state == "success" else "GitHub integration failed",
            "url": f"https://github.com/{config['repository']}/actions/runs/{batch[1]}"}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_):
        return None  # Never forward the GitHub credential to another URL.


def dispatch(server, job):
    """Only a current, authenticated upstream request can launch the fixed workflow."""
    if (job.get("name") != REQUEST_JOB or job.get("status") != "running"
            or job.get("pipeline", {}).get("project_id") != PROJECT):
        raise PermissionError("expected an active upstream request job")
    request_id = positive_id(job["id"])
    pipeline_id = positive_id(job["pipeline"]["id"])
    pipeline = server.api.get(f"/projects/{PROJECT}/pipelines/{pipeline_id}")
    jobs = pipeline_jobs(server.api, pipeline_id)
    if (pipeline["status"] in {"canceled", "canceling"}
            or jobs.get(REQUEST_JOB, {}).get("id") != request_id
            or not gate_jobs(jobs)):
        raise PermissionError("request is canceled or superseded")
    checks = [value for name, value in jobs.items()
              if name == "checks" or name.startswith("checks: [")]
    if (not checks or any(value.get("status") != "success" for value in checks)
            or any(jobs.get(name, {}).get("status") != "success"
                   for name in ("test", "build", "prepare-mediawiki"))):
        raise PermissionError("request prerequisites have not passed")
    repository = server.config["repository"]
    if repository not in {REPOSITORY, "addshore/testing"}:
        raise ValueError("unsupported GitHub repository")
    # A job executes on one host. Persist acknowledgement across client retries
    # and service restarts; a retried GitLab job gets a new request ID.
    with server.dispatch_lock:
        marker = server.state / f"{repository.replace('/', '-')}-{pipeline_id}-{request_id}"
        if not marker.exists():
            ref = pipeline.get("ref", "")
            merge_request = re.fullmatch(r"refs/merge-requests/([1-9][0-9]*)/(?:head|merge)", ref)
            label = f"MR !{merge_request[1]}" if merge_request else ref or f"pipeline {pipeline_id}"
            payload = {"ref": "main", "inputs": {
                "project_id": str(PROJECT), "job_id": str(request_id),
                "job_name": REQUEST_JOB, "job_status": "success", "run_label": label}}
            request = urllib.request.Request(
                f"https://api.github.com/repos/{repository}/actions/workflows/gitlab-integration.yml/dispatches",
                data=json.dumps(payload).encode(), method="POST", headers={
                    "Authorization": "Bearer " + server.dispatch_token,
                    "Accept": "application/vnd.github+json",
                    "Content-Type": "application/json", "User-Agent": "mwcli-native-ci"})
            with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
                if response.status not in {200, 204}:
                    raise ValueError("unexpected GitHub dispatch response")
            marker.touch(mode=0o600)
    return {"state": "success", "message": "GitHub integration dispatched"}


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass  # Never log job tokens or request headers.

    def do_GET(self):
        self.handle_call(False)

    def do_POST(self):
        self.handle_call(True)

    def handle_call(self, launching):
        code, result = 503, {"state": "pending", "message": "Result service temporarily unavailable"}
        try:
            if self.path != ("/dispatch" if launching else "/status"):
                raise PermissionError("unknown endpoint")
            token = self.headers.get("JOB-TOKEN", "")
            if not token or len(token) > 4096:
                raise PermissionError("missing job identity")
            request = urllib.request.Request(API + "/job", headers={"JOB-TOKEN": token})
            with urllib.request.urlopen(request, timeout=20) as response:
                job = json.load(response)
            result = dispatch(self.server, job) if launching else check(self.server.api, job, self.server.config)
            code = 200
        except PermissionError:
            code, result = 403, {"state": "failed", "message": "Job identity or prerequisites were rejected"}
        except urllib.error.HTTPError as error:
            if error.code in {401, 403}:
                code, result = 403, {"state": "failed", "message": "Service credential or job authentication was rejected"}
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
    credentials = Path(os.environ["CREDENTIALS_DIRECTORY"])
    socket = Path("/run/mwcli-wait/status.sock")
    socket.unlink(missing_ok=True)
    with Server(str(socket), Handler) as server:
        server.config = config
        server.api = GitLab((credentials / "gitlab-read-token").read_text().strip())
        server.dispatch_token = (credentials / "github-dispatch-token").read_text().strip()
        if not server.dispatch_token:
            raise ValueError("GitHub dispatch credential is empty")
        server.dispatch_lock = threading.Lock()
        server.state = Path(os.environ["STATE_DIRECTORY"])
        os.chmod(socket, 0o666)  # Each caller must authenticate with its own CI_JOB_TOKEN.
        server.serve_forever()
