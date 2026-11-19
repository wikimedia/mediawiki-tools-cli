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
mw config set phabricator.public_only true
```

In this mode, authenticated task access requires a public view policy in
Wikimedia’s public S1 Space. Hidden tasks appear as `[NON PUBLIC]` with ID,
status, priority, and timestamps in JSON. Opening, commenting on, or changing
those tasks is blocked. Task creation is blocked because its visibility is
unknown before submission.

## AI agents and human overrides

Agent detection and `--agent` enable public-only mode. Check detection with
`mw debug agent`. A human can use `--allow-non-public` outside agent mode.
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
`mw phab api` can access non-public data even with `--public-only`. Paste reads
and project metadata are also outside the task filter. Enforce isolation by
keeping credentials and human caches outside the agent’s reach.

## Confirming changes

Use `--dry-run` to preview supported changes without contacting Phabricator.
Task creation, paste creation or editing, and profile removal prompt for
confirmation or require `--yes` in scripts. When agent policy requires
approval, show the preview when available, ask the user, then rerun with
`--yes`. Task visibility checks still apply.

## Use Phabricator from an MCP client

```sh
mw phab mcp
mw mcp --services phab,wiki
mw mcp --profile phab=work
```

MCP starts anonymously with public tasks, comments, and wiki pages. A
Phabricator profile can return non-public task summaries. Add `--allow-paste`
with a profile to read account-visible pastes.
`--deferred-services` authorizes services for later activation with
`mw_enable_service`. Search returns CLI JSON with up to 50 results by default.
Repeat the filters and pass `cursor.after` as `after` for the next page.
