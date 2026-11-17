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

The CLI-wide `agent.policy` setting controls approval, initially only for
Phabricator commands. Missing or invalid command hints fail closed in agent mode;
an invalid policy value is an error.

| Policy | Ordinary reads | Targeted writes | High-risk or unknown operations |
|---|---|---|---|
| `permissive` | Allow | Allow | Ask user |
| `balanced` (default) | Allow | Ask user | Ask user |
| `cautious` | Ask user | Ask user | Ask user |

```sh
mw config set agent.policy balanced
```

High-risk commands are credential login, local profile removal, the interactive
mutation shell and every raw API call (including read methods). Secret input,
secret output and unknown data access also require approval under every policy.
Changing one task's status is a targeted write, not automatically high-risk just
because it overwrites a field. Supported offline `--dry-run` previews require no
approval under any policy.

When approval is required, the command fails before its handler runs and tells
the agent to explain the exact target, inputs and risks to the user. For supported
writes, the agent should first run `--dry-run` and show the preview. Only after
the user confirms should the agent rerun that same operation with `--yes`.
Approval is per invocation, not saved; `--yes=false` does not approve.
`--yes` is a cooperative assertion, not verified evidence that the agent asked.
Agents should not change `agent.policy` or their detection environment to avoid
asking the user.

Detection uses [Vercel's detect-agent](https://github.com/vercel/detect-agent),
plus a Copilot SDK session-marker fallback. `AI_AGENT` identifies custom agents;
upstream nonempty-variable checks treat even `0` and `false` as detection.
Check the result with the hidden diagnostic command:

```sh
mw debug agent
# {"agent":"github_copilot_vscode_agent","is_agent":true}
```

If task visibility access is denied, the agent must ask a human to rerun the operation in their
own terminal outside agent mode. A human can override the public-only default
with `--allow-non-public` or `--public-only=false`. `--allow-non-public` is rejected
in agent mode; it does not grant the agent permission to retry. `--yes` satisfies
command approval only, not task visibility checks. Task creation remains blocked
in agent mode even with approval because its visibility cannot be verified in
advance. Public-task writes can proceed when the selected policy permits them.

Without agent mode or a public-only setting, normal account permissions apply.

## Previewing and confirming changes

One-shot task changes (including the legacy top-level aliases), paste writes,
and `auth default`/`auth remove` support:

- `--dry-run`: preview supplied changes without applying them or accessing
  credentials. Task/paste previews do not contact Phabricator or resolve remote
  names, permissions or policies; they are not a guarantee that applying will
  succeed. Profile previews read local profile metadata only.
- `--yes`: assert the user's approval when required by agent policy, and skip
  terminal confirmation for profile removal. Never overrides server permissions
  or task visibility restrictions.
- `--no-interaction`: never prompt. Explicit single-task/paste writes and default
  profile selection still run. Profile removal requires `--yes` (or `--dry-run`)
  instead of assuming confirmation. For agents, chat approval via `--yes` is
  independent from terminal interaction; `--no-interaction` grants no consent.

```sh
mw phab task assign T12345 me --dry-run
mw phab task status T12345 resolved --dry-run
mw phab auth remove --profile bot --dry-run

# A human outside agent mode can explicitly apply the change
mw phab task assign T12345 me --no-interaction

# After showing the preview and obtaining user confirmation, an agent can apply
mw phab task assign T12345 me --yes --no-interaction
```

For humans outside agent mode, specifying the task/paste and changes is sufficient
intent: creation, editing and legacy one-shot mutations apply without confirmation.
Selecting an explicit default profile also applies directly. Removing a local
credential profile still requires confirmation or `--yes`; cancellation makes
no changes. Agent restrictions apply regardless of these confirmation defaults.
The interactive mutation shell applies its internal changes immediately and
requires an interactive terminal. Agent approval for this high-risk command
covers the session, not individual changes; prefer one-shot commands for
narrowly scoped approvals. Login and raw API calls have no generic dry-run.
For raw API approval, explain the method and full payload's effects and
data-access scope rather than guessing safety from names such as `.search`.
Never expose credential values while asking for approval.

## Limitations

- These are local guardrails, not an authorization boundary. Detection is
  best-effort; agents with shell access can bypass it or use credentials directly.
  Isolate privileged credentials and human caches for enforceable restrictions.
- **`mw phab api` is unfiltered**, even with task access flags and in agent mode.
  Every agent call requires approval via `--yes` under every policy. Approval
  explicitly covers the raw method and payload, including potential non-public
  data access; it does not make the API equivalent to filtered task commands.
  Non-task objects, including paste reads and project metadata, also remain
  outside the public-task filter. The command policy does not verify visibility.
- Stronger approval requires a trusted execution boundary, for example an agent
  runner or MCP host/server enforcing grants. MCP annotations alone do not
  enforce approval. Isolate credentials to prevent bypass through another client.
- Public task content is not scanned and may itself contain confidential details.
