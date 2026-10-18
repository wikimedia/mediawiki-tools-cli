# CI

GitLab runs checks and builds on shared Wikimedia runners. Docker integration tests run in [GitHub Actions](https://github.com/wikimedia/mediawiki-tools-cli/actions/workflows/gitlab-integration.yml); Cloud VPS jobs wait for their results.

## When CI runs

CI runs for MRs (including drafts), `main`, tags and weekly image updates. You can also start `main` pipelines through the UI or API. New commits cancel older interruptible jobs.

MR pipelines test a temporary merge with the target branch. If the target advances, start a new pipeline; retries reuse the existing merge.

## Caches and scheduling

`prepare-mediawiki` shares a clean snapshot with integration tests. It refreshes weekly or when preparation scripts change. Use GitLab's **Clear runner caches** to force a refresh.

Preview release builds run alongside integration. Documentation builds and tagged releases wait for integration to pass.

## GitHub integration

After checks, builds and MediaWiki preparation pass, `integration-github-request` starts [the GitHub workflow](.github/workflows/gitlab-integration.yml) with the pipeline's prepared inputs. No webhook is needed.

Results appear in GitLab's `external` stage with links to GitHub logs. `integration-github` blocks downstream jobs until all tests pass.

### Native platform workflows

Separate `gitlab-integration-<platform>.yml` workflows support `linux-arm64`,
`macos-arm64`, and `windows-amd64`. They build and run tests from the exact
GitLab source. The automated pipeline uses the Ubuntu AMD64 workflow.

Native dispatches require a successful project 16 job named
`integration-github-<platform>-<test>-request`, supplied as `job_id` and
`job_name`. The required `test` input selects a shell test from the GitLab
source; results appear as `github/<platform>/<test>`. Preparation and reporting
hold the callback credentials on separate runners from builds and tests.

### Credentials

Install the workflows and helpers on the official GitHub repository's `main` branch before using the bridge.

| Credential | Permission | Storage | Expires (UTC) |
| --- | --- | --- | --- |
| `GITLAB_STATUS_TOKEN` | GitLab project 16, Maintainer, `api` | Official GitHub Actions secret | 2027-11-22 |
| Result reader | GitLab project 16, Reporter, `read_api` | Cloud VMs: `/etc/gitlab-runner-wait/status-token`, root-only | 2027-11-22 |
| GitHub dispatch token | Official repository, Actions read/write | Cloud VMs: `/etc/gitlab-runner-wait/github-token`, root-only | 2027-10-19 |
| Waiting runner authentication | Accept and report assigned jobs | VM: `/etc/gitlab-runner-wait/config.toml`, root-only | No expiry configured |

Create tokens in [GitLab settings](https://gitlab.wikimedia.org/repos/releng/cli/-/settings/access_tokens) or with the command below. Replace `YYYY-MM-DD` with an allowed expiry; record it above when rotating. For the reader, use a separate name, `access_level: 20` and `read_api`:

```sh
mw gl api projects/16/access_tokens --hostname gitlab.wikimedia.org --method POST --input - <<'JSON'
{"name":"mwcli native GitHub results","access_level":40,"scopes":["api"],"expires_at":"YYYY-MM-DD"}
JSON
```

Save the writer token at the hidden prompt:

```sh
gh secret set GITLAB_STATUS_TOKEN --repo wikimedia/mediawiki-tools-cli
```

Keep the writer's returned `user_id` for [pool setup](#waiting-pools). Host tokens stay outside MR jobs; [masked CI variables remain readable by scripts](https://docs.gitlab.com/ci/variables/#cicd-variable-security). Root and privileged Docker jobs can access host tokens.

Create a GitHub fine-grained token for `wikimedia/mediawiki-tools-cli` with **Actions: read and write**. Install it on both VMs below.

Enable **Pipelines must succeed** in GitLab's merge checks.

Verify an MR pipeline's external results and waiting job, then test `main`:

```sh
mw gl api projects/16/pipeline --hostname gitlab.wikimedia.org --method POST --raw-field ref=main
```

### Waiting pools

Each VM provides 50 slots tagged `mwcli-wait`, locked to this project. Services: `gitlab-runner-wait` (jobs) and `mwcli-wait-status` (dispatch and results).

Set `INSTANCE_NAME`, `INSTANCE_ID` and `SSH_USER` from the [inventory](#cloud-vps-workers), or [create a VM](#add-a-cloud-vps-worker). On a new VM, install packages:

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  --script tools/ci/runner.sh --sudo -- install
```

On your workstation, create a runner (skip for existing pools):

```sh
mw gl api user/runners --hostname gitlab.wikimedia.org --method POST \
  --raw-field runner_type=project_type --field project_id=16 \
  --raw-field description="${INSTANCE_NAME}-wait" --raw-field tag_list=mwcli-wait \
  --field locked=true --field run_untagged=false --field maximum_timeout=3600
```

Keep the returned `glrt-...` token for registration. From the repository root, copy files and connect:

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  -- sudo install -d -m 0755 /opt/mwcli-wait

tar -C tools/ci -cf - github_status.py wait-github.py wait-status.py \
  wait-runner.toml gitlab-runner-wait.service mwcli-wait-status.service | \
  mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
    --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
    -- sudo tar --no-same-owner -xf - -C /opt/mwcli-wait

mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org --tty
```

On the VM, enter a root shell:

```sh
sudo -i
set +x
umask 077
install -d -m 0700 /etc/gitlab-runner-wait
install -d -m 0750 /var/lib/gitlab-runner-wait
```

Register once. **Skip for existing pools** to avoid duplicate entries.

```sh
(set -o noclobber; printf 'concurrent = 50\ncheck_interval = 3\nshutdown_timeout = 30\n' > /etc/gitlab-runner-wait/config.toml) || exit 1
read -rsp 'Waiting runner authentication token: ' runner_token; echo
CI_SERVER_TOKEN="$runner_token" gitlab-runner register --non-interactive \
  --config /etc/gitlab-runner-wait/config.toml \
  --template-config /opt/mwcli-wait/wait-runner.toml \
  --url https://gitlab.wikimedia.org --name "$(hostname -s)-wait" \
  --executor docker --docker-image python:3.13-alpine
unset runner_token
```

In that root shell, save credentials and the writer bot's `user_id`. Skip unchanged credentials on existing VMs:

```sh
read -rsp 'GitLab read_api token: ' reader_token; echo
printf '%s\n' "$reader_token" > /etc/gitlab-runner-wait/status-token
unset reader_token
read -rsp 'GitHub Actions dispatch token: ' dispatch_token; echo
[[ -n $dispatch_token ]] || exit 1
printf '%s\n' "$dispatch_token" > /etc/gitlab-runner-wait/github-token
unset dispatch_token
read -rp 'Writer bot user_id: ' writer_id
[[ $writer_id =~ ^[1-9][0-9]*$ ]] || exit 1
printf '{"bot_id":%s,"repository":"wikimedia/mediawiki-tools-cli"}\n' "$writer_id" > /etc/mwcli-wait-status.json
chmod 0644 /etc/mwcli-wait-status.json
```

Install and start the services:

```sh
install -m 0644 /opt/mwcli-wait/*.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now mwcli-wait-status gitlab-runner-wait
systemctl restart mwcli-wait-status
gitlab-runner verify --config /etc/gitlab-runner-wait/config.toml
systemctl status gitlab-runner-wait mwcli-wait-status
```

Verify an `integration-github` job receives results. The [registration template](tools/ci/wait-runner.toml) pins jobs to CPU 3; adjust for other hardware. Edit existing pools in `/etc/gitlab-runner-wait/config.toml`; the template applies only at registration.

### Retries and recovery

Use GitHub's **Re-run failed jobs** to repeat failed tests and automatically retry the GitLab waiting job. Retrying only the waiting job rechecks existing results. Start a new pipeline if inputs have expired (one day).

For a missed dispatch, run **GitLab integration** in GitHub with project `16`, the successful request job ID and status `success`. Rerun failed reporting jobs to retry callbacks.

## Cloud VPS workers

Workers run in the `mwcli` Cloud VPS project.

| Instance | OpenStack ID | Docker volume |
| --- | --- | --- |
| `gitlab-runner-addshore-1018` | `b4fe2ce4-1a42-4668-9d07-c9fb6be16fc9` | `mwcli-ci-0002` |
| `gitlab-runner-addshore-1019` | `ea86a663-eee4-4ed7-9962-d7f339800b5c` | `mwcli-ci-0001` |

Both VMs use Trixie, 4 CPUs, 8 GiB RAM, 20 GB root disks and 40 GiB Docker volumes at `/var/lib/docker`.

[GitLab runner settings](https://gitlab.wikimedia.org/repos/releng/cli/-/settings/ci_cd#js-runners-settings) · [Cloud VPS usage dashboard](https://grafana.wmcloud.org/d/0g9N-7pVz/cloud-vps-project-board?orgId=1&var-project=mwcli)

## Add a Cloud VPS worker

Run from the repository root in one Bash session. Skip resources that already exist.

### 1. Authenticate

Use an `mw` build with `cloudvps` commands; see [development builds](README.md#using-a-development-build).

In [Horizon](https://horizon.wikimedia.org/), select `mwcli`, then **Identity → Application Credentials → Create Application Credential**. Save its ID and secret.

```sh
mw cloudvps auth add --project mwcli --id <application-credential-id> --secret <application-credential-secret>
mw cloudvps auth check --project mwcli
mw gitlab auth status --hostname gitlab.wikimedia.org
```

Skip `auth add` if configured. Use `mw gitlab auth login --hostname gitlab.wikimedia.org` if needed. Runner creation requires `create_runner`; editing requires `manage_runner`.

Cloud VPS credentials are stored in an owner-only plaintext config. `--secret` also exposes them in shell history and process arguments.

### 2. Create the VM and attach storage

Check resources and choose unused names:

```sh
mw cloudvps compute list --project mwcli
mw cloudvps volume list --project mwcli

INSTANCE_NAME=gitlab-runner-addshore-1020
VOLUME_NAME=mwcli-ci-0003
SSH_USER=addshore
```

Set `SSH_USER` to your Wikimedia shell username. Check example IDs with `mw cloudvps image list`, `flavor list` and `network list`, each using `--project mwcli`. This example uses Debian 13, 4 CPUs and 8 GiB RAM.

Create the VM:

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

For reused storage, skip creation and set `VOLUME_ID` to an **available** volume's ID. Its existing data will be preserved.

```sh
mw cloudvps volume attach "$VOLUME_ID" --project mwcli --server "$INSTANCE_ID" --wait
```

`--wait` does not wait for SSH or cloud-init. If creation fails, inspect existing resources before retrying to avoid duplicates.

### 3. Prepare the volume

From your workstation, prepare the volume and check its mount:

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  --tty -- sudo wmcs-prepare-cinder-volume

mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  -- findmnt /var/lib/docker
```

Mount the attached volume at `/var/lib/docker`. Preserve reused filesystems; stop if formatting is proposed. See [disk preparation](https://wikitech.wikimedia.org/wiki/Help:Adding_Disk_Space_to_Cloud_VPS_instances).

Continue with [waiting-pool setup](#waiting-pools).

## Maintenance

Set `INSTANCE_ID` from the inventory and `SSH_USER` to your shell username:

```sh
mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  -- sudo systemctl status gitlab-runner-wait mwcli-wait-status

mw cloudvps compute ssh "$INSTANCE_ID" --project mwcli \
  --user "$SSH_USER" --jump primary.bastion.wmcloud.org \
  -- sudo journalctl -u mwcli-wait-status -n 30 --no-pager
```

For updates, repeat file copying and service installation from [pool setup](#waiting-pools). Preserve the runner config, token files and `/var/lib/mwcli-wait` dispatch records.
