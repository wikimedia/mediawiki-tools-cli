# Outbound HTTP identity

Use `cli.HTTPUserAgent` for CLI-owned HTTP requests. For a new HTTP client, use
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
| Recipe downloads/readiness, doctor, gateway requests | Shared client or explicit standard header |
| MediaWiki clone/submodule, source updates and Gerrit fetch subprocesses | Versioned CLI identity through `GIT_HTTP_USER_AGENT`; explicit user overrides preserved |
| Gerrit documentation scraper, docs publishing and image-update registry clients | Shared client or explicit standard header |
| Python CI requests and release uploads | Same standard identity, defined locally outside the Go module |
| Embedded dashboard readiness/quick-tunnel requests | Same standard identity; dashboard builds standalone |
| Embedded glab | Wikimedia CLI identity first on factory-created clients, alongside `glab/<version>` and platform details |
| Docker SDK, including named contexts and remote daemons | Standard CLI identity through the SDK's `WithUserAgent` option |

Toolforge uses `useragent.SetApplication(cli.HTTPUserAgent)` before creating
commands. Its existing transport adds that identity first and retains generated
SDK identifiers, including through custom TLS and SSH tunnels. Native Kubernetes
REST and WebSocket/SPDY clients set the identity in `rest.Config.UserAgent`,
preserving client-go's transports. Arbitrary curl
requests also carry the Toolforge default product identity. The dependency is
pinned to the UA child branch of `embed-command-tree`; the original embedding
MR remains unchanged. The same UA commit is available separately on upstream
`main` for independent review.

glab's own authentication paths can create clients outside the factory. Its
upstream UA renderer includes the CLI identity as a fallback there, though the
upstream renderer retains glab-first ordering.

External Git, SSH, curl,
Composer, npm and container image pulls manage their own network traffic; the
CLI HTTP transport does not intercept those subprocesses.

The standard constant lives in `internal/cli/app.go`. Standalone Python, shell
and embedded-dashboard copies should be updated if that identity changes.
