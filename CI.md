# CI

Continuous integration runs in Wikimedia GitLab. Shared Wikimedia runners handle
Go checks, builds, basic CLI tests, get-code tests and MediaWiki preparation.
By default, Docker integration jobs use the `dind` tag, shared by persistent Cloud
VPS runners and temporary GitHub-hosted GitLab workers. Native GitHub integration
is available behind `GITHUB_NATIVE_INTEGRATION=1`; its rollout is described below.

Full CI requires the project's configured runners and is not available in forks.

## When CI runs

CI runs for merge requests, including drafts, pushes to `main`, tag pushes and the
weekly image-update schedule. Feature-branch pushes do not create branch pipelines;
open a draft MR to get CI while working on a change.

New commits automatically cancel interruptible jobs from older pipelines for the
same MR or `main` branch, including running tests and builds.

Before the build and tests start, `prepare-merge` fetches the current target branch
and merges it into a temporary checkout of the MR commit. Conflicts fail this early
job. Otherwise, a Git bundle passes that exact merged commit to the remaining jobs,
including security scans. Your source branch is not changed or pushed.

If the target branch advances afterwards, rerun the pipeline to test the newer
combination. Retrying only a test job uses the existing merge artifact; start a new
pipeline to refresh it. This does not provide GitLab's native merged-results or
merge-train guarantees at merge time.

## Pipeline caches and scheduling

Basic CLI integration tests, get-code tests and MediaWiki preparation use shared
runners. Docker tests use the `dind` runners. Preview release builds run alongside
integration; tagged releases still wait for all integration tests.

Go jobs let Make install only the pinned tools they need. Module, compiler and
linter caches live under `.cache/`, with separate cache keys per job and Go version.

`prepare-mediawiki` caches a compressed, clean MediaWiki snapshot and passes it as
a pipeline artifact to the four tests that need it. Its Wikimedia image includes
PHP, Node and Composer, which run directly without a Docker daemon. Tests
never refresh or upload their modified MediaWiki trees. The get-code test fetches
its own code. Snapshots refresh weekly and when the preparation scripts change;
use **Clear runner caches** in
GitLab to force an earlier refresh. Each runner can rebuild its own cache on a miss,
so no shared cache service is required.

## Native GitHub integration (opt in)

[Native test runs](https://github.com/wikimedia/mediawiki-tools-cli/actions/workflows/gitlab-integration.yml)
run the four Docker tests as ordinary GitHub Actions jobs, using each Ubuntu VM's
Docker daemon. Cloud VPS pools run only the small waiting jobs; they do not run
the native integration tests. Shared Wikimedia runners handle the other GitLab jobs.

`integration-github-request` finishes after checks, unit tests, the binary build
and MediaWiki preparation. Its successful Job event dispatches
[the native workflow](.github/workflows/gitlab-integration.yml). GitHub downloads
the binary, clean MediaWiki snapshot and exact merged Git bundle from that
pipeline. Every test uses those same inputs; it does not build another revision.

Four individual results appear in GitLab's `external` stage:

- `github/docker-general`
- `github/all-databases`
- `github/mysql-cycle`
- `github/repoclient`

Each links to its GitHub job and full log. GitLab does not store the native logs.
A fresh reporting VM posts each result when that test finishes. The test VMs have
no callback token; reporting uses trusted GitHub workflow code and GitHub's job
conclusions, rather than files supplied by the tested branch.

`integration-github` is a normal job on the `mwcli-wait` pool. It starts
automatically and polls every 15 seconds, using no checkout or artifacts. There
is no manual action. The VM's local result reader authenticates the job with its
short-lived `CI_JOB_TOKEN`, then reads GitLab using a separate read-only project
token. That long-lived token stays outside job containers. The reader checks the
current pipeline/request, configured reporting bot, four results, common GitHub
run/attempt and exact merged commit before allowing success. It exposes only a
local Unix socket, not a public HTTP endpoint.

Each waiting job exits successfully when all four tests pass, or fails once a
complete authenticated batch contains a failure. Missing, mixed or invalid
results cannot pass; the wait times out after 40 minutes. Documentation builds
and tagged releases wait for this job; preview release builds still run alongside
integration. GitHub tests check for GitLab cancellation every 30 seconds.
Duplicate dispatches share a concurrency group and completed requests are
skipped. No scheduled polling workflow is used.

### Enable the bridge

Keep the current runners and webhook until these steps are complete:

1. Merge this change into GitLab `main`, then confirm the mirror has copied
   `gitlab-integration.yml` and `gitlab-integration-test.yml` to GitHub `main`.
   Dispatchable workflows must exist on the GitHub default branch.
2. Create a project access token in [GitLab access tokens](https://gitlab.wikimedia.org/repos/releng/cli/-/settings/access_tokens):
   name **mwcli native GitHub results**, role **Maintainer**, scope **api**.
   Maintainer preserves automatic retry on protected `main`/release refs; this
   is a broad project write credential, not a status-only permission. Save it
   directly to the official GitHub repo with the secure prompt:

   ```sh
   gh secret set GITLAB_STATUS_TOKEN --repo wikimedia/mediawiki-tools-cli
   ```

   Alternatively, use `mw gl api` to create the token and pipe its value directly
   into `gh`. This prints only metadata, including the writer bot's `user_id`, to
   stderr; the secret goes directly to GitHub. Run once per intended rotation:

   ```sh
   set -o pipefail
   mw gl api projects/16/access_tokens --hostname gitlab.wikimedia.org --method POST --input - <<'JSON' | jq -er '({id,user_id,name,expires_at}|tojson|stderr|empty), (.token // error("Missing token"))' | gh secret set GITLAB_STATUS_TOKEN --repo wikimedia/mediawiki-tools-cli
   {"name":"mwcli native GitHub results","access_level":40,"scopes":["api"],"expires_at":"2027-11-22"}
   JSON
   ```

   `mw gl` must be authenticated as a project Maintainer; `gh` needs permission
   to manage the official repo's secrets. Keep shell tracing (`set -x`) disabled.
3. Create a separate token named **mwcli native CI result reader**, with
   **Reporter** role and **read_api** scope in the same GitLab UI. Keep its value
   ready for the secure VM setup prompts; do not save it as a GitLab CI variable.
   Obtain the writer bot ID from the CLI response above, or list its metadata:

   ```sh
   mw gl api projects/16/access_tokens --hostname gitlab.wikimedia.org | jq '.[] | select(.name == "mwcli native GitHub results") | {id,user_id,expires_at,scopes}'
   ```

   Using that `user_id`, install the reader on both VMs. Each command prompts for
   the same read-only token without echoing it:

   ```sh
   python3 tools/ci/configure-wait-pool.py addshore@gitlab-runner-addshore-1018.mwcli.eqiad1.wikimedia.cloud --bot-id WRITER_USER_ID --read-token
   python3 tools/ci/configure-wait-pool.py addshore@gitlab-runner-addshore-1019.mwcli.eqiad1.wikimedia.cloud --bot-id WRITER_USER_ID --read-token
   ```

   The Cloud SSH key must be loaded. To create the reader using `mw` instead of
   the UI, run the following once. Its response includes the secret: keep it
   private and use it only in the setup prompts.

   ```sh
   mw gl api projects/16/access_tokens --hostname gitlab.wikimedia.org --method POST --input - <<'JSON'
   {"name":"mwcli native CI result reader","access_level":20,"scopes":["read_api"],"expires_at":"2027-11-22"}
   JSON
   ```
4. The longest lifetime previously accepted by this instance is **400 days**:
   **22 November 2027** when created on **18 October 2026**. Adjust the dates in
   these examples when creating tokens later. Administrators can shorten the
   limit; [GitLab documents the default 365-day and optional 400-day limits](https://docs.gitlab.com/user/project/settings/project_access_tokens/).
   Arrange rotation before the accepted expiry. Keep the separate trial token
   until rollout is verified, then revoke it. Tokens are displayed only at creation;
   GitHub cannot return the value of a saved Actions secret.
5. Set the GitHub Actions repository variable **GITLAB_NATIVE_ENABLED** to `true`.
6. Add a Job events webhook in
   [GitLab webhooks](https://gitlab.wikimedia.org/repos/releng/cli/-/hooks), using:

   ```text
   https://api.github.com/repos/wikimedia/mediawiki-tools-cli/actions/workflows/gitlab-integration.yml/dispatches
   ```

   Enable **Job events**, keep SSL verification enabled and use this custom body:

   ```json
   {"ref":"main","inputs":{"project_id":"{{project_id}}","job_id":"{{build_id}}","job_name":"{{build_name}}","job_status":"{{build_status}}"}}
   ```

   Add the custom header **Authorization** with value `Bearer YOUR_GITHUB_TOKEN`.
   The GitHub fine-grained token needs access only to this repository and
   **Actions: read and write**. The existing dispatch token can be reused if it
   still has those permissions. GitLab sends all job events; the workflow accepts
   only successful `integration-github-request` jobs from project 16 before
   allocating a runner, then checks the event against the GitLab API.
7. Enable **Pipelines must succeed** in
   [GitLab merge request settings](https://gitlab.wikimedia.org/repos/releng/cli/-/settings/merge_requests)
   so failed or blocked pipelines prevent merging.
8. Set the nonsecret GitLab CI/CD variable **GITHUB_NATIVE_INTEGRATION** to `1`,
   available to MR pipelines with **Protected** unchecked. Open a draft MR or push
   a new commit to an existing MR to exercise the native path. Check that four
   results appear, each log link works, and the gate finishes automatically.
   Test a failing integration change too: the gate and downstream jobs must not
   pass. Then verify a `main` pipeline before retiring the old setup.

This mode is restricted to upstream project 16. Fork pipelines retain the legacy
`dind` jobs and need their own runners; the callback credential is never shared
with forks.

### Trial before merging into main

Use `addshore/testing` to test an unmerged GitLab draft MR. Its copies of the
native workflows select that repository explicitly; the production workflows
continue to accept only `wikimedia/mediawiki-tools-cli`.

On a separate GitLab trial branch, set these YAML variables:

```yaml
GITHUB_NATIVE_INTEGRATION: "1"
GITHUB_INTEGRATION_REPOSITORY: "addshore/testing"
```

Publish the native workflow and helper files to `addshore/testing`'s default
branch. Set its **GITLAB_NATIVE_ENABLED** variable and **GITLAB_STATUS_TOKEN**
secret, and configure the VM readers with that token's bot ID and
`--repository addshore/testing`.
Use a separate temporary token for the trial. It needs the same Maintainer role
and `api` scope as production; the old read-only queue token is insufficient.

Push the GitLab trial branch and open a draft MR. Leave the production webhook
alone. First dispatch **GitLab native integration** manually in `addshore/testing`
with the successful `integration-github-request` job ID. That exercises artifact
transfer, all four native tests, individual GitLab statuses and the automatic
gate. A separate Job events webhook pointing at `addshore/testing` can then test
automatic dispatch. The existing webhook body and permissions apply.

Do not change the project-wide **GITHUB_NATIVE_INTEGRATION** variable for this
trial: only the trial branch should select the new mode. Before merging the
production branch, configure both VM readers with the production writer bot ID
and `--repository wikimedia/mediawiki-tools-cli`. Disable the trial
webhook and revoke the temporary token when the test is finished.

### Lightweight waiting pools

Both Cloud VMs have a separate 50-slot pool, giving up to 100 small waits:

| VM | Waiting runner |
| --- | --- |
| `gitlab-runner-addshore-1018` | [#1551](https://gitlab.wikimedia.org/repos/releng/cli/-/runners/1551) |
| `gitlab-runner-addshore-1019` | [#1550](https://gitlab.wikimedia.org/repos/releng/cli/-/runners/1550) |

These runners are tagged `mwcli-wait`, locked to this project, with untagged jobs
disabled. Each uses its own `gitlab-runner-wait` systemd service and root-only
`/etc/gitlab-runner-wait/config.toml`. The existing Docker runner retains its two
slots and its original configuration. The result reader is a separate
`mwcli-wait-status` service, running as a restricted dynamic user with a systemd
credential loaded from root-only `/etc/gitlab-runner-wait/status-token`.

To create a pool on an existing runner, update its scripts, or select the reporting
bot/repository, run these full commands from this checkout:

```sh
python3 tools/ci/configure-wait-pool.py addshore@gitlab-runner-addshore-1018.mwcli.eqiad1.wikimedia.cloud --bot-id WRITER_USER_ID
python3 tools/ci/configure-wait-pool.py addshore@gitlab-runner-addshore-1019.mwcli.eqiad1.wikimedia.cloud --bot-id WRITER_USER_ID
```

Add `--read-token` to securely prompt for an **existing** `read_api` token instead
of creating another one. GitHub does not allow retrieving a saved Actions secret;
you need its original value. Add `--repository addshore/testing` for the trial.
The commands upload the tracked scripts, create a project runner only if needed,
and reuse its existing authentication token on subsequent runs. A new registration
requires your `mw gl` account to be allowed to create project runners. Use `--mw`
if the CLI executable is at another path.

Only `python:3.13-alpine` is accepted. Jobs have no privileged mode, Docker socket,
cache or sidecar services. The two read-only mounts contain the trusted waiting
client and the local result socket; neither contains a long-lived token. Each job
has a 64 MiB memory limit, a 0.1 CPU limit and a 64-process limit. All waits on each
host share one CPU core; 50 is a concurrency ceiling, not a CPU reservation. Jobs
use `GIT_STRATEGY: none` and download no artifacts. Docker's shared bridge avoids
allocating a separate network for every wait.

### Credentials and risks

| Credential | Permission | Stored in |
| --- | --- | --- |
| `GITLAB_STATUS_TOKEN` | Project 16, Maintainer, `api` | Official GitHub Actions secret; trusted preparation/report jobs only |
| VM result reader | Project 16, Reporter, `read_api` | Both Cloud VMs, root-only file loaded as a service credential |
| GitHub dispatch token | Official GitHub repo, Actions read/write | GitLab webhook Authorization header |
| Waiting runner authentication | Pick up/submit assigned jobs | Each VM's root-only runner configuration |
| `CI_JOB_TOKEN` | Short-lived job identity | GitLab injects it into its running job |

The writer can alter more than statuses: compromise permits project changes at
Maintainer level. Native test VMs receive neither long-lived GitLab token; fresh
reporting VMs execute workflow code from the trusted GitHub revision and read actual
Actions conclusions. Protect changes to that workflow and keep its write token out
of tested code, artifacts and logs. The dispatch token can start Actions workflows;
it does not grant GitLab access.

The reader cannot post successful statuses or change the repository, but can read
project metadata, logs and artifacts. Root on either Cloud VM can obtain it. These
VMs also host privileged Docker integration jobs, so those jobs remain a route to
host compromise; file permissions do not isolate a credential from host root.
The small waiting containers themselves have no Docker access or privileged mode.
Longer expiry increases the time a stolen token remains useful; rotate the writer
and reader before their printed expirations and revoke superseded trial tokens.

### Retry, retire and roll back

For a failing native test, use **Re-run failed jobs** in the matching GitHub run.
The reporter keeps successful results from earlier attempts of that same run,
refreshes the four GitLab statuses and retries a failed waiting job automatically.
Do not retry only the GitLab gate to rerun tests: it only verifies existing results.
For a failed or blocked run, use **Re-run all jobs** to repeat preparation and all
tests. An already passed gate is left unchanged; start a new GitLab pipeline to
repeat a successful pipeline. Inputs expire after
one day; start a new GitLab pipeline if its artifacts have expired.

If the webhook was missed, run **GitLab native integration** manually in GitHub
with project `16`, the successful `integration-github-request` job ID, and status
`success`. If reporting failed, rerun the failed GitHub reporting job. A canceled
GitLab pipeline is not revived. Cancel its matching GitHub run directly if needed.

After successful MR and `main` trials, disable the old queue webhook and set
**GITLAB_WORKERS_ENABLED** to `false`. Pause the two old Docker runner registrations
and the old GitHub project runner in GitLab. Keep the `-wait` registrations and
both VM result-reader services running. Keep their tokens and VM data until the rollback
window has passed, then unregister the old runners and remove the old GitHub
secrets **GITLAB_RUNNER_TOKEN** and **GITLAB_API_READ_TOKEN**. Native integration
uses neither GitHub secret. Keep **GITLAB_STATUS_TOKEN**, the VM reader credential
and the new webhook token. If you reused an old read token for the VM reader, do
not revoke that token while the reader uses it.

To roll back, set **GITHUB_NATIVE_INTEGRATION** to `0`, restore the old webhook and
**GITLAB_WORKERS_ENABLED**, and unpause the `dind` runners. Start a new pipeline;
changing the variable does not alter jobs in an existing pipeline.

## GitHub workers (legacy mode)

See [actual worker runs](https://github.com/wikimedia/mediawiki-tools-cli/actions/workflows/gitlab-worker.yml)
in GitHub Actions.

The GitLab webhook dispatches [the queue workflow](.github/workflows/gitlab-workers.yml).
It rechecks pending jobs and accepts jobs requiring exactly the `dind` tag.
Other status events are skipped. When no matching job remains, the queue workflow
cancels itself. Otherwise it dispatches [the worker workflow](.github/workflows/gitlab-worker.yml)
using its built-in GitHub token; no additional secret or webhook change is needed.

Select **GitLab CI worker** in GitHub Actions to see only worker runs. Queue checks
appear separately under **GitLab CI queue**. Worker runs are named with their
GitLab job ID. Separate jobs can start workers in parallel, subject to GitHub's
concurrency limits; duplicate events for the same job share a concurrency group.
There is no scheduled GitHub polling.

Keep one or both Cloud VPS runners online with the `dind` tag. They poll GitLab
normally and can handle jobs even if the webhook or GitHub is unavailable. GitLab
assigns each job to an available matching runner; the shared tag does not give
Cloud VPS priority. The webhook can still start a GitHub worker for any pending
`dind` job. If Cloud VPS takes it first, the queue check skips it, or an already
started worker exits after its idle timeout.

All temporary workers reuse one project runner authentication token. Each VM generates a
unique runner manager ID and removes only its own manager afterwards. A worker
exits after one job or after 120 seconds without work.

Queue-check regression tests can run locally:

```sh
python3 -m unittest discover -s .github/scripts -p 'test_*.py'
```

### Tokens

The tokens for this setup will need rotating.

- GITLAB_RUNNER_TOKEN doesn't expire (Currently for https://gitlab.wikimedia.org/repos/releng/cli/-/runners/1549)
- GITLAB_API_READ_TOKEN expires 21 November 2027 (Currently https://gitlab.wikimedia.org/repos/releng/cli/-/settings/access_tokens `GitHub official worker queue`)
- GH token in GL webhook expires 18 October 2027 (Currently https://github.com/settings/personal-access-tokens/21319900)

## Custom runners (legacy mode)

Custom runners run in the `mwcli` Cloud VPS project.

Reported on 2026-10-17:

| Instance | OpenStack ID | Docker volume |
| --- | --- | --- |
| `gitlab-runner-addshore-1018` | `b4fe2ce4-1a42-4668-9d07-c9fb6be16fc9` | `mwcli-ci-0002` |
| `gitlab-runner-addshore-1019` | `ea86a663-eee4-4ed7-9962-d7f339800b5c` | `mwcli-ci-0001` |

All runners use Trixie, 4 CPUs, 8 GiB RAM, a 20 GB root disk, and a 40 GiB Docker
volume mounted at `/var/lib/docker`, with a local registry mirror and GitLab Runner
configured for two concurrent jobs.

[GitLab runner settings](https://gitlab.wikimedia.org/repos/releng/cli/-/settings/ci_cd#js-runners-settings)
· [Cloud VPS usage dashboard](https://grafana.wmcloud.org/d/0g9N-7pVz/cloud-vps-project-board?orgId=1&var-project=mwcli)

## Add another runner

Run these commands from the repository root on your workstation, in the same
Bash session. Only volume preparation is interactive on the VM; the rest of
its configuration is handled by [tools/ci/runner.sh](tools/ci/runner.sh).
Do not repeat creation for an existing VM, volume, or GitLab runner.

### 1. Authenticate

Use a build with the experimental `cloudvps` commands (`mw cloudvps --help`).
See [README.md](README.md#using-a-development-build) for running a development build.

For Cloud VPS, select `mwcli` in [Horizon](https://horizon.wikimedia.org/), then
**Identity → Application Credentials → Create Application Credential**. Choose
an appropriate expiry and permissions, and save the ID and secret when displayed.

```sh
mw cloudvps auth add --project mwcli --id <application-credential-id> --secret <application-credential-secret>
mw cloudvps auth check --project mwcli
mw gitlab auth status --hostname gitlab.wikimedia.org
```

Skip `auth add` when credentials are already configured. If GitLab is not
configured, use `mw gitlab auth login --hostname gitlab.wikimedia.org`.
The GitLab account and access token must permit creating project runners
(`create_runner` scope); changing runner details requires `manage_runner`.
See [Cloud VPS credentials](https://wikitech.wikimedia.org/wiki/Help:Using_OpenStack_APIs#Application_Credentials)
and [GitLab runner creation](https://docs.gitlab.com/api/users/#create-a-runner-linked-to-a-user).

Cloud VPS secrets are currently stored as plaintext in mwcli's config file,
created with owner-only permissions. `--secret` can expose them in shell history
and process arguments; config output and trace logs can also reveal them.
OS credential store support and hidden input for `auth add` are not implemented.

### 2. Create the VM and attach storage

Check existing resources, then choose unused names. Instance numbering is a
convention, not an automatic sequence; the names below are examples.

```sh
mw cloudvps compute list --project mwcli
mw cloudvps volume list --project mwcli

INSTANCE_NAME=gitlab-runner-addshore-1020
VOLUME_NAME=mwcli-ci-0003
SSH_USER=addshore
```

Change `SSH_USER` to your Wikimedia shell username. Discover current resources
with `mw cloudvps image list`, `flavor list`, and `network list`, each with
`--project mwcli`. The following IDs were available on 2026-10-17:

| Resource | Name | ID |
| --- | --- | --- |
| Image | `debian-13.0-trixie` | `09ac7914-8b26-41c2-b5ab-9c2319ca9e9b` |
| Flavor | `g4.cores4.ram8.disk20` | `acdad2e5-6221-4f3c-ac17-ec6fa13bf523` |
| Network | `VXLAN/IPv6-dualstack` | `d8a16ddf-c01f-4f22-8b67-8ed18b4b1b45` |

Create the VM, capturing its ID directly:

```sh
INSTANCE_ID=$(mw cloudvps compute create "$INSTANCE_NAME" --project mwcli \
  --image 09ac7914-8b26-41c2-b5ab-9c2319ca9e9b \
  --flavor acdad2e5-6221-4f3c-ac17-ec6fa13bf523 \
  --network d8a16ddf-c01f-4f22-8b67-8ed18b4b1b45 \
  --security-group default --wait --output jq --format .id)
```

For fresh storage:

```sh
VOLUME_ID=$(mw cloudvps volume create "$VOLUME_NAME" --project mwcli \
  --size 40 --wait --output jq --format .id)
```

Alternatively, skip volume creation and set `VOLUME_ID` to an existing
**available** volume's ID. This preserves its Docker data, including containers
and registry credentials. `mwcli-ci-0001` and `mwcli-ci-0002` are already in use.

```sh
mw cloudvps volume attach "$VOLUME_ID" --project mwcli --server "$INSTANCE_ID" --wait
```

Stop if any operation fails. `--wait` defaults to ten minutes (override with
`--timeout 20m`) and does not wait for SSH or cloud-init. Creation prints an ID
before waiting; inspect the resource with `compute list`, `compute get`, or
`volume get` before retrying, rather than creating duplicates.

### 3. Prepare the volume

From your workstation, run the volume preparation tool on the VM, then check
its mount. Omit `--user` and `--jump` if your SSH configuration supplies them:

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  --tty -- sudo wmcs-prepare-cinder-volume

mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  -- findmnt /var/lib/docker
```

Choose the attached 40 GiB data volume and mount it at `/var/lib/docker`.
For reused storage, preserve the existing filesystem and stop if formatting
is proposed. See [Cloud VPS disk preparation](https://wikitech.wikimedia.org/wiki/Help:Adding_Disk_Space_to_Cloud_VPS_instances).

### 4. Create the GitLab runner through the CLI

Skip this step if the GitLab runner already exists. Resolve the project's ID,
then create a project runner with the matching description, `mwcli` and `dind` tags,
untagged jobs disabled, and locking enabled:

```sh
PROJECT_ID=$(mw gitlab api projects/repos%2Freleng%2Fcli \
  | python3 -c 'import json, sys; print(json.load(sys.stdin)["id"])')

mw gitlab api --method POST user/runners \
  --raw-field runner_type=project_type \
  --field project_id="$PROJECT_ID" \
  --raw-field description="${INSTANCE_NAME}-docker" \
  --raw-field tag_list=mwcli,dind \
  --field run_untagged=false \
  --field locked=true
```

The response contains the new GitLab runner ID and a `glrt-...` authentication
token that is only returned at creation. Keep the token ready for the setup
prompt; do not share this response or paste the token into command arguments.
The runner's GitLab **description** is separate from the local `--name` used
when registering it. Setting the description above keeps both names consistent.

### 5. Run the setup script

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  --script tools/ci/runner.sh --sudo --tty -- setup
```

The CLI uploads the local Bash script to a private temporary file, runs it,
and removes the remote copy. `--sudo` requires passwordless sudo; `--tty`
allows credential prompts. The script:

- Requires Debian 13 and a mounted `/var/lib/docker`; never formats or prunes it.
- Installs Docker and GitLab Runner, retaining the classic Docker image store.
- Reuses an existing registry, or creates one after prompting for Docker Hub credentials.
- Prompts for the GitLab runner token without echoing it, unless already registered locally.
- Sets both concurrency limits to two and configures the host and Docker-in-Docker mirror.
- Verifies runner registration and prints storage, container, and service status.

Docker Hub tokens need public repository read access. A separate `docker login`
is unnecessary. Secrets are passed without command arguments; Docker retains
mirror credentials in container metadata, and GitLab Runner retains its token
in root-only configuration. A changed runner config is backed up once to
`/etc/gitlab-runner/config.toml.before-mwcli`.

For staged setup, use `install` followed by `configure` instead of `setup`.
The script can be rerun without duplicating the local runner entry or mirror;
it preserves unrelated configuration. Docker image, privileged mode, and
certificate volume defaults are set at registration; reruns preserve any later
customisations to those settings. Setup restarts Docker and GitLab Runner,
so run it while the host is not serving CI jobs.

### 6. Verify an integration job

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  --script tools/ci/runner.sh --sudo -- status
```

Check GitLab shows the runner online, then run or retry an integration job in
a merge request pipeline. Confirm its log names `${INSTANCE_NAME}-docker`. Registration
and a running service alone do not verify Docker-in-Docker or image pulls.

## Existing runners and maintenance

For `1019`, set the variables below, then run the full SSH command from step 6
with `status` or `configure` as needed. Do not recreate its resources.

```sh
INSTANCE_NAME=gitlab-runner-addshore-1019
INSTANCE_ID=ea86a663-eee4-4ed7-9962-d7f339800b5c
SSH_USER=addshore
```

To change an existing runner's GitLab display description:

```sh
mw gitlab api --method PUT runners/1547 \
  --raw-field description="${INSTANCE_NAME}-docker" --silent
```

To reclaim unused Docker data on an idle runner:

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  -- sudo docker system prune -af
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  --tty -- sudo docker volume prune
```

If space is still exhausted, inspect the registry cache. Removing and recreating
its container and cache volume discards cached images; pulls will repopulate it.
