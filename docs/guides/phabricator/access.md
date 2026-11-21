# Phabricator access

## Public reads

With no selected or default profile or legacy token, `task list`, `task view`,
`read`, `comments`, and `project view` read public Wikimedia Phabricator pages
anonymously. Writes, paste reads, and raw API calls need credentials. See
[Phabricator login](../authentication.md#phabricator-login-and-account-profiles).

```sh
wm phab task view T440478
wm phab task list --project a_wikimedia_cli --column Backlog
```

Repeat `--project` or use comma-separated values to match every project. Column
search needs one visible workboard. Use an explicit username in anonymous
searches. `me` requires a profile.

## Public-only reads

Pass `--public-only` to read Wikimedia Phabricator anonymously even when you have
a default profile. This works with task list, task view, read, comments, and
project view. For example:

```sh
wm --public-only phab task list --project a_wikimedia_cli
```

Set `access.public_only` to `true` to make public-only access the CLI default.
The setting also covers wiki page source and info. Commands without public-only
support return an error. Pass `--public-only=false` to use your normal profile for
one invocation. The older `phabricator.public_only` setting applies to Phabricator
when the global setting is unset.

## Preview and confirm changes

Task creation, multi-field task edits, and paste writes show the supplied fields
with `--dry-run`, including text read from a file or stdin. A task move preview
uses the selected profile to look up the task, project, and destination column
without performing the move.

```sh
wm phab task move T12345 Backlog --project a_wikimedia_cli --dry-run
```

Task creation, multi-field task edits, paste writes, and profile removal prompt
for confirmation. Use `--yes` to confirm explicitly in scripts. Single-purpose
task updates such as setting a status or moving a task apply directly.

## Use Phabricator from an MCP client

See the [MCP guide](../mcp.md) for public reads and explicit profile bindings.
