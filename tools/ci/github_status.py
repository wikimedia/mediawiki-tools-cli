"""GitLab identities and status checks shared by the native GitHub CI bridge."""

import json
import os
import re
import time
import urllib.error
import urllib.parse
import urllib.request

API = "https://gitlab.wikimedia.org/api/v4"
PROJECT = 16
REPOSITORY = "wikimedia/mediawiki-tools-cli"
REQUEST_JOB = "integration-github-request"
GATE_JOB = "integration-github"
SUMMARY_CONTEXT = "github/integration"


SUPPORTED_PLATFORMS = {"linux-amd64", "linux-arm64", "macos-arm64", "windows-amd64"}


def gate_test(name):
    """Read the platform/test declared by a GitLab parallel matrix gate job."""
    if not name.startswith(GATE_JOB + ":"):
        return None
    match = re.fullmatch(r"integration-github: \[([a-z][a-z0-9-]*), ([a-z][a-z0-9-]{0,63})\]", name)
    if not match or match[1] not in SUPPORTED_PLATFORMS:
        raise ValueError("invalid GitLab integration matrix job")
    platform, test = match.groups()
    context = test if platform == "linux-amd64" else f"{platform}/{test}"
    return {"platform": platform, "test": test, "name": context}


def gate_jobs(jobs):
    return {name: job for name, job in jobs.items() if name == GATE_JOB or gate_test(name) is not None}


def integration_repository():
    """Explicitly allow the official deployment and the isolated rollout trial."""
    repository = os.environ.get("GITHUB_INTEGRATION_REPOSITORY", REPOSITORY)
    if repository not in {REPOSITORY, "addshore/testing"}:
        raise ValueError("unsupported GitHub integration repository")
    return repository


def positive_id(value):
    if not re.fullmatch(r"[1-9][0-9]*", str(value)):
        raise ValueError("expected a positive numeric ID")
    return int(value)


def commit_sha(value):
    if not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", str(value)):
        raise ValueError("expected a full commit SHA")
    return value


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, new_url):
        redirected = super().redirect_request(request, fp, code, message, headers, new_url)
        if urllib.parse.urlparse(new_url).scheme != "https":
            raise ValueError("refusing an insecure artifact redirect")
        if urllib.parse.urlparse(new_url).netloc != urllib.parse.urlparse(API).netloc:
            redirected.remove_header("Private-token")
        return redirected


class GitLab:
    def __init__(self, token=""):
        self.token = token

    def request(self, path, data=None, destination=None):
        if not path.startswith("/") or ".." in path or "://" in path:
            raise ValueError("expected a GitLab API path")
        headers = {"User-Agent": "a-wikimedia-cli-ci"}
        if self.token:
            headers["PRIVATE-TOKEN"] = self.token
        body = None
        if data is not None:
            headers["Content-Type"] = "application/json"
            body = json.dumps(data).encode()
        for attempt in range(4):
            try:
                req = urllib.request.Request(API + path, data=body, headers=headers)
                with urllib.request.build_opener(SafeRedirect()).open(req, timeout=60) as response:
                    if destination is not None:
                        with open(destination, "wb") as output:
                            while chunk := response.read(1024 * 1024):
                                output.write(chunk)
                        return None
                    return json.load(response)
            except urllib.error.HTTPError as error:
                if error.code not in {409, 429, 500, 502, 503, 504} or attempt == 3:
                    raise
                error.close()
                time.sleep(2 ** attempt)
            except (urllib.error.URLError, TimeoutError):
                if attempt == 3:
                    raise
                time.sleep(2 ** attempt)

    def get(self, path):
        return self.request(path)

    def post(self, path, data):
        return self.request(path, data=data)

    def all(self, path):
        separator = "&" if "?" in path else "?"
        page = 1
        while True:
            items = self.get(f"{path}{separator}per_page=100&page={page}")
            yield from items
            if len(items) < 100:
                return
            page += 1


def latest_jobs(jobs):
    result = {}
    for job in jobs:
        if job["name"] not in result or job["id"] > result[job["name"]]["id"]:
            result[job["name"]] = job
    return result


def pipeline_jobs(api, pipeline_id):
    return latest_jobs(api.all(f"/projects/{PROJECT}/pipelines/{positive_id(pipeline_id)}/jobs"))


def statuses(api, sha, pipeline_id):
    query = urllib.parse.urlencode({"pipeline_id": positive_id(pipeline_id), "all": "false"})
    return list(api.all(f"/projects/{PROJECT}/repository/commits/{commit_sha(sha)}/statuses?{query}"))


def description(request_id, run_id, attempt, tested_sha):
    return (
        f"request={positive_id(request_id)};run={positive_id(run_id)};"
        f"attempt={positive_id(attempt)};tested={commit_sha(tested_sha)}"
    )


def parse_description(value):
    match = re.fullmatch(
        r"request=([1-9][0-9]*);run=([1-9][0-9]*);attempt=([1-9][0-9]*);tested=([0-9a-f]{40}|[0-9a-f]{64})",
        value or "",
    )
    if not match:
        raise ValueError("missing native integration identity")
    return match.groups()


def verify_status(record, context, *, bot_id, request_id, tested_sha, repository=REPOSITORY):
    """Authenticate a trusted controller result for an expected context."""
    if record.get("name") != context or record.get("allow_failure", False):
        raise ValueError("missing required integration result")
    if record.get("author", {}).get("id") != positive_id(bot_id):
        raise ValueError("result was not posted by the configured GitHub CI bot")
    batch = parse_description(record.get("description"))
    if batch[0] != str(positive_id(request_id)) or batch[3] != commit_sha(tested_sha):
        raise ValueError("result belongs to another request or source snapshot")
    prefix = f"https://github.com/{repository}/actions/runs/{batch[1]}"
    url = record.get("target_url", "")
    if url != prefix and not url.startswith(prefix + "/"):
        raise ValueError("result has an unexpected GitHub run URL")
    return batch


def verify_summary(record, **identity):
    return verify_status(record, SUMMARY_CONTEXT, **identity)
