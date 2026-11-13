# Outbound HTTP identity

Use `cli.HTTPUserAgent()` for CLI-owned HTTP requests. It returns
`a-wikimedia-cli/<version> (https://a-wikimedia-cli.toolforge.org/)` using the
running release's version, a source build's Git description, or `dev` when no
build metadata is available. Evaluate it when constructing clients or requests,
after application startup supplies the version. For a new HTTP client, use
`cli.NewHTTPClient()` and configure its timeout and redirect policy as needed.
For an injected client, `cli.WithHTTPUserAgent(client)` preserves those settings
and its transport. Both preserve explicit User-Agent headers and identify
redirected requests without changing global HTTP defaults.

| Request path | Identity configuration |
| --- | --- |
| Release lookup and release notes | `internal/gitlab/wikimedia.go`: GitLab SDK `UserAgent` |
| Update binary/artifact downloads, including redirects | `internal/updater/root.go`: shared HTTP client |
| Gerrit commands, auth and change lookup | All `gerrit.NewClient` calls receive the shared HTTP client |
| Cloud VPS authentication, reauthentication and service APIs | `commands/cloudvps/client.go`: prepend CLI identity to Gophercloud UA before authentication |
| Cloud VPS discovery, wiki APIs/uploads, Toolhub, codesearch, REST APIs, EventStreams, quips and telemetry | Explicit standard header or SDK UA |
| Wiki file downloads and rendered previews, including redirects | `commands/wiki/file_download.go`: shared HTTP client |
| Recipe downloads/readiness, doctor, gateway requests | Shared client or explicit standard header |
| MediaWiki clone/submodule, source updates and Gerrit fetch subprocesses | Versioned CLI identity through `GIT_HTTP_USER_AGENT`; explicit user overrides preserved |
| Gerrit documentation scraper, docs publishing and image-update registry clients | Shared client or explicit standard header |
| Python CI requests and release uploads | `a-wikimedia-cli-ci (https://a-wikimedia-cli.toolforge.org/)` |
| Embedded dashboard readiness/quick-tunnel requests | Standard versioned identity; `MWCLI_VERSION` is supplied through the development environment |
| Embedded glab | Wikimedia CLI identity first on factory-created clients, alongside `glab/<version>` and platform details |
| Docker SDK, including named contexts and remote daemons | Standard CLI identity through the SDK's `WithUserAgent` option |

Toolforge uses `useragent.SetApplication(cli.HTTPUserAgent())` before creating
commands. Its existing transport adds that identity first and retains generated
SDK identifiers, including through custom TLS and SSH tunnels. Native Kubernetes
REST and WebSocket/SPDY clients set the identity in `rest.Config.UserAgent`,
preserving client-go's transports. Arbitrary curl
requests also carry the Toolforge default product identity.

GitLab's local authentication commands use the shared identity for credential
checks; factory-created clients retain the embedded glab version suffix.

External Git, SSH, curl,
Composer, npm and container image pulls manage their own network traffic; the
CLI HTTP transport does not intercept those subprocesses.

The standard formatter lives in `internal/cli/app.go`. Standalone maintenance
tools use `a-wikimedia-cli-maint (https://a-wikimedia-cli.toolforge.org/)`,
following the CI identity's unversioned format. This includes the realm-list
updater, which refreshes the trusted CentralAuth wiki endpoint list in
`internal/wikiauth/wikimedia.txt`, and the Codex asset updater.

Documentation tooling retains
`a-wikimedia-cli-docs/1.0 (https://a-wikimedia-cli.toolforge.org/)`.
The standalone Python, shell, and embedded-dashboard URL copies should be
updated if the project URL changes.
