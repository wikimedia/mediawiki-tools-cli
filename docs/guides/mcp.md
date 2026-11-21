# Use Wikimedia CLI tools over MCP

`wm mcp` lets an MCP client read public Phabricator tasks, Gerrit changes,
GitLab merge requests, and wiki pages. The client launches `wm` as a local
stdio server. By default, it uses anonymous access even if you have saved
profiles. Queries and identifiers go to the Wikimedia service you select.

## Connect a client

Run the line for your client. Use the full path to `wm` if the client cannot
find it on its PATH. If your installation is named `mw`, use `mw` in the same configuration.

```sh
# GitHub Copilot CLI
copilot mcp add wikimedia -- wm mcp

# Codex CLI
codex mcp add wikimedia -- wm mcp

# Claude Code
claude mcp add --transport stdio wikimedia -- wm mcp
```

For Copilot in VS Code, add the server to `~/.copilot/mcp-config.json`
(Copilot Global).

```json
{
  "mcpServers": {
    "wikimedia": {
      "type": "stdio",
      "command": "wm",
      "args": ["mcp"]
    }
  }
}
```

See your client's setup docs for configuration details.
[Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) ·
[Codex CLI](https://learn.chatgpt.com/docs/extend/mcp?surface=cli) ·
[Claude Code](https://code.claude.com/docs/en/mcp) ·
[VS Code](https://code.visualstudio.com/docs/agents/reference/mcp-configuration)

To try it, ask your client to use the Wikimedia MCP server to summarize
public Phabricator task `T250000` and its comments. Filter the command catalog
by **Has MCP tool** to see the operations, CLI counterparts, and data each
tool sends and reads.
[Command catalog](https://a-wikimedia-cli.toolforge.org/manual/catalog/)

Phabricator task search returns up to 50 results by default. For the next
page, keep the same filters and pass `cursor.after` as `after`. For GitLab,
pass `next_page` as `page` for diffs or `next_cursor` as `after` for discussions.
For Gerrit, pass the change details’ `current_revision` as `revision` to keep
files, diffs, commit text, and comments on the same patch set.

## Choose which services to expose

Use `--services` in the client's launch command to select from `phab`, `gerrit`,
`gitlab`, and `wiki`.

```sh
wm mcp --services phab,gerrit
wm mcp --services phab --deferred-services wiki
```

With the second example, `mw_discover_tools` lists wiki tools and
`mw_enable_service` enables them during the session. The client may need to
refresh its tool list. `wm phab mcp` is a shortcut for a Phabricator-only server.

## Use a Phabricator account

Pass `--profile phab=work` with `--public-only=false` to use the saved `work` profile
for task search. Results can include summaries of restricted tasks visible to the
account. Task details, comments, and project lookup stay anonymous.

```sh
wm mcp --services phab --profile phab=work --public-only=false
```

Add `--allow-paste` to expose account-visible paste reads. See
[Phabricator login](authentication.md#phabricator-login-and-account-profiles).
