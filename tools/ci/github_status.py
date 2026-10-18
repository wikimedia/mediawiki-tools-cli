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
TESTS = {
    "docker-general": "test-docker-general.sh",
    "all-databases": "test-docker-mw-all-dbs.sh",
    "mysql-cycle": "test-docker-mw-mysql-cycle.sh",
    "repoclient": "test-docker-recipe-repoclient.sh",
}
CONTEXTS = [f"github/{name}" for name in TESTS]


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
        headers = {"User-Agent": "mwcli-native-ci"}
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


def verify_results(records, *, bot_id, request_id, tested_sha, repository=REPOSITORY):
    """Require every test to pass for the same bot/run/snapshot."""
    bot_id = positive_id(bot_id)
    expected_request = str(positive_id(request_id))
    commit_sha(tested_sha)
    by_name = {}
    for record in records:
        name = record.get("name")
        if name in CONTEXTS and (name not in by_name or record["id"] > by_name[name]["id"]):
            by_name[name] = record
    identity = None
    for name in CONTEXTS:
        record = by_name.get(name)
        if record is None or record.get("status") != "success" or record.get("allow_failure", False):
            raise ValueError(f"{name} has not passed")
        if record.get("author", {}).get("id") != bot_id:
            raise ValueError(f"{name} was not posted by the configured GitHub CI bot")
        batch = parse_description(record.get("description"))
        if batch[0] != expected_request or batch[3] != tested_sha:
            raise ValueError(f"{name} belongs to another request or source snapshot")
        if identity is not None and identity != batch:
            raise ValueError("integration results belong to different GitHub runs or attempts")
        identity = batch
        prefix = f"https://github.com/{repository}/actions/runs/{batch[1]}"
        url = record.get("target_url", "")
        if url != prefix and not url.startswith(prefix + "/"):
            raise ValueError(f"{name} has an unexpected GitHub run URL")
    return identity
