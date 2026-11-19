# Use Wikimedia CLI tools over MCP

`mw mcp` lets an MCP client read public Phabricator tasks, Gerrit changes,
GitLab merge requests, and wiki pages. The client launches `mw` as a local
stdio server. By default, it uses anonymous access even if you have saved
profiles. Queries and identifiers go to the Wikimedia service you select.

## Connect a client

Run the line for your client. Use the full path to `mw` if the client cannot
find it on its PATH.

```sh
# GitHub Copilot CLI
copilot mcp add wikimedia -- mw mcp

# Codex CLI
codex mcp add wikimedia -- mw mcp

# Claude Code
claude mcp add --transport stdio wikimedia -- mw mcp
```

For Copilot in VS Code, add the server to `~/.copilot/mcp-config.json`
(Copilot Global).

```json
{
  "mcpServers": {
    "wikimedia": {
      "type": "stdio",
      "command": "mw",
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
by **Has MCP tool** to see the available operations and their CLI counterparts.
[Command catalog](https://a-wikimedia-cli.toolforge.org/manual/catalog/)

Phabricator task search returns up to 50 results by default. For the next
page, keep the same filters and pass `cursor.after` as `after`.

## Choose which services to expose

Use `--services` in the client's launch command to select from `phab`, `gerrit`,
`gitlab`, and `wiki`.

```sh
mw mcp --services phab,gerrit
mw mcp --services phab --deferred-services wiki
```

With the second example, `mw_discover_tools` lists wiki tools and
`mw_enable_service` enables them during the session. The client may need to
refresh its tool list. `mw phab mcp` is a shortcut for a Phabricator-only server.

## Use a Phabricator account

Pass `--profile phab=work` to use the saved `work` profile for task search.
Search can return non-public task summaries as permitted by the profile and
the CLI's access policy. Task details, comments, and project lookup stay
public.

```sh
mw mcp --services phab --profile phab=work
```

Add `--allow-paste` only when you want the client to read pastes visible to that
account. See [Phabricator login](authentication.md#phabricator-login-and-account-profiles).
