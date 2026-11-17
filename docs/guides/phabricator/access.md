# Phabricator task access and agent guardrails

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

Detected agents automatically use public-only task access. You can also enable
it explicitly with `--agent`. Neither `--agent=false` nor `--public-only=false`
disables automatic detection.

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
edit confirmation, not access checks.

Without agent mode or a public-only setting, normal account permissions apply.

## Limitations

- These are local guardrails, not an authorization boundary. Detection is
  best-effort; agents with shell access can bypass it or use credentials directly.
  Isolate privileged credentials and human caches for enforceable restrictions.
- **`mw phab api` is unfiltered**, even with access flags. Non-task objects,
  including pastes and project metadata, are also outside this filter.
- Public task content is not scanned and may itself contain confidential details.
