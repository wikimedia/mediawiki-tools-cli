# CI

Continuous integration runs in Wikimedia GitLab. Shared Wikimedia runners handle
Go checks, builds, basic CLI tests, get-code tests and MediaWiki preparation.
Docker integration jobs use the `dind` tag, shared by persistent Cloud VPS runners
and temporary GitHub-hosted workers. Each GitHub worker connects back to GitLab,
runs one job and uploads its logs and artifacts there.

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

## GitHub workers

The worker workflow is [`.github/workflows/gitlab-workers.yml`](.github/workflows/gitlab-workers.yml).
A GitLab job-event webhook dispatches it when a job becomes pending. A separate
queue-check job rechecks the job's status and accepts jobs requiring exactly the
`dind` tag. Other status events are skipped. Separate jobs can start workers in
parallel, subject to GitHub's concurrency limits; duplicate events for the same
job share a concurrency group. There is no scheduled GitHub polling.

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

## Custom runners

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
