# Phabricator access and agent guardrails

Limit accidental access to non-public tasks, either by default or when using an
AI agent. For login and profiles, see [authentication](../authentication.md#phabricator-login-and-account-profiles).

## Public-only tasks

```sh
mw phab task list --author me --order newest --status all --limit 20 --public-only --output table
mw phab task view T12345 --public-only

# Make public-only access the default
mw config set phabricator.public_only true
```

Only tasks on `https://phabricator.wikimedia.org` with `fields.policy.view:
public` in the known public S1 Space are accepted. Other servers, Spaces,
policies or missing metadata are treated as unverified. S1 is allowlisted, not
dynamically revalidated.

For non-public or unverified tasks:

- Lists show `[NON PUBLIC]`, retaining ID, status, priority and created/modified
  timestamps (timestamps are available in JSON/jq). JSON marks these rows with
  `redacted: true`. Task existence, ordering and search/board placement remain
  visible, but content and other task metadata are hidden.
- Opening tasks, reading comments and making changes are blocked. Placeholders
  cannot be opened in the interactive browser.
- Task creation is blocked because visibility cannot be verified beforehand.

Restricted commands bypass disk caches and check current task metadata before
comments and edits. Existing cache files are left untouched.

## AI agents and human overrides

Detected agents automatically use public-only task access and a command policy
based on the command's capability hints. You can also enable these restrictions
explicitly with `--agent`. Neither `--agent=false` nor `--public-only=false`
disables automatic detection.

Agents can read permitted data and preview supported mutations with `--dry-run`.
Actual task/paste changes, local profile changes, credential login, raw API calls
and the interactive mutation shell require a human rerun outside agent mode.
This also applies to public tasks and idempotent writes. Read-only `browse` and
account status/profile listing remain available, subject to their usual checks.
Missing or invalid command hints fail closed in agent mode.

Detection uses [Vercel's detect-agent](https://github.com/vercel/detect-agent),
plus a Copilot SDK session-marker fallback. `AI_AGENT` identifies custom agents;
upstream nonempty-variable checks treat even `0` and `false` as detection.
Check the result with the hidden diagnostic command:

```sh
mw debug agent
# {"agent":"github_copilot_vscode_agent","is_agent":true}
```

If access is denied, the agent must ask a human to rerun the operation in their
own terminal outside agent mode. A human can override the public-only default
with `--allow-non-public` or `--public-only=false`. `--allow-non-public` is rejected
in agent mode; it does not grant the agent permission to retry. `--yes` only skips
confirmation, not agent policy or access checks.

Without agent mode or a public-only setting, normal account permissions apply.

## Previewing and confirming changes

One-shot task changes (including the legacy top-level aliases), paste writes,
and `auth default`/`auth remove` support:

- `--dry-run`: preview supplied changes without applying them or accessing
  credentials. Task/paste previews do not contact Phabricator or resolve remote
  names, permissions or policies; they are not a guarantee that applying will
  succeed. Profile previews read local profile metadata only.
- `--yes`: skip the normal confirmation, but never override agent policy,
  permissions or task visibility restrictions.
- `--no-interaction`: never prompt. Without `--yes` or `--dry-run`, changes fail
  rather than assume confirmation.

```sh
mw phab task assign T12345 me --dry-run
mw phab task status T12345 resolved --dry-run
mw phab auth remove --profile bot --dry-run

# A human outside agent mode can explicitly apply the change
mw phab task assign T12345 me --yes --no-interaction
```

Scripts using the older immediate task-mutation commands or profile mutations
must now supply `--yes`. Cancellation makes no changes. The interactive mutation
shell still applies its internal changes immediately, so it is unavailable in
agent mode and requires an interactive terminal; use one-shot commands for
preview/confirmation workflows. Login has no dry-run: it remains a separate human
credential-setup operation. The raw API has neither a generic dry-run nor a
generic confirmation because arbitrary Conduit methods cannot be safely
classified; humans must review their method and payload themselves.

## Limitations

- These are local guardrails, not an authorization boundary. Detection is
  best-effort; agents with shell access can bypass it or use credentials directly.
  Isolate privileged credentials and human caches for enforceable restrictions.
- **`mw phab api` is unfiltered outside agent mode**, even with task access flags;
  it is refused in agent mode. Non-task objects, including paste reads and project
  metadata, remain outside the public-task filter. The command policy does not
  verify their visibility.
- Public task content is not scanned and may itself contain confidential details.
