# Phabricator access and agent guardrails

## Public reads

With no selected or default profile or legacy token, `task list`, `task view`,
`read`, `comments`, and `project view` read public Wikimedia Phabricator pages
anonymously. Writes, paste reads, and raw API calls need credentials. See
[Phabricator login](../authentication.md#phabricator-login-and-account-profiles).

```sh
mw phab task view T440478
mw phab task list --project a_wikimedia_cli --column Backlog
```

Repeat `--project` or use comma-separated values to match every project. Column
search needs one visible workboard. Use an explicit username in anonymous
searches. `me` requires a profile.

## Public-only tasks

Use `--public-only` for one invocation or set it as the default.

```sh
mw config set access.public_only true
```

In this mode, authenticated task access requires a public view policy in
Wikimedia’s public S1 Space. Known restricted tasks there appear as
`[NON PUBLIC]` with ID, status, priority, and timestamps in CLI JSON. Search
omits tasks whose visibility is unknown and withholds that page’s cursors.
Opening, commenting on, or changing restricted tasks is blocked. Task creation
is blocked because its visibility is unknown before submission.

The setting also covers wiki page source and info. Other commands without
public-only support return an error. The older `phabricator.public_only`
setting still applies to Phabricator task commands when the global setting
is unset.

## AI agents and human overrides

Agent detection and `--agent` enable public-only mode for Phabricator tasks.
Check detection with `mw debug agent`. A human can use `--allow-non-public`
outside agent mode.
Set Phabricator’s approval policy with `mw config set agent.policy POLICY`.
Supported offline previews always run without approval.

| `agent.policy` | Ask the user before |
|---|---|
| `permissive` | High-risk operations |
| `balanced` (default) | Writes and high-risk operations |
| `cautious` | All commands |

High-risk operations include login, profile removal, the mutation shell, raw
API calls, and commands that accept secrets, reveal secrets, or have unknown
data access.
Raw API calls, paste reads, and project metadata can expose non-public data
when public-only mode is off. Enforce isolation by keeping credentials and
human caches outside the agent’s reach.

## Confirming changes

Use `--dry-run` to preview supported changes without contacting Phabricator.
Task creation, paste creation or editing, and profile removal prompt for
confirmation or require `--yes` in scripts. When agent policy requires
approval, show the preview when available, ask the user, then rerun with
`--yes`. Task visibility checks still apply.

## Use Phabricator from an MCP client

See the [MCP guide](../mcp.md) for anonymous access, named profiles, and paste reads.
