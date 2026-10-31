# Credentials and password managers

On the first interactive Gerrit, Cloud VPS, Phabricator, GitLab or wiki credential save, the CLI checks for an OS keyring and KeePassXC, then asks where to store credentials. Plaintext configuration is also an option. The choice becomes the default after a successful save. Configuration holds the account and a reference to the secret. Linux uses Secret Service over D-Bus, macOS uses Keychain, and Windows uses Credential Manager. Commands read credentials only when needed; unrelated commands do not unlock the keyring.

On native Windows, the storage picker shows Windows Credential Manager once. In WSL, OS keyring means Linux Secret Service and Windows Credential Manager means the Windows store, so both remain available when detected.

```sh
mw gerrit auth login
mw cloudvps auth login
mw phabricator auth login
```

Use hidden prompts or `--password-stdin`, `--secret-stdin` and `--token-stdin` rather than putting secrets in command arguments or shell history. A password-manager command can pipe its output into these flags. Do not enable shell tracing (`set -x`) around secrets.

## Existing credentials

Inspect saved credential metadata and storage locations without unlocking providers or revealing values:

```sh
mw config secrets list
mw config secrets list --json
mw config secrets list --check
mw config secrets list --verify
mw config secrets list phabricator.profiles.wikimedia.key --check --verify
```

The inventory includes known credential references, plaintext config secrets, and the local KeePassXC pairing file. The compact table shows key, provider, comparison and authentication results. Add `--details` to show service, account, endpoint, status and storage location below the table. Account mismatches and missing references remain visible in the summary. A credential with both a vault link and a retained plaintext value has a separate row for each copy; their values are not compared. JSON rows include a `source` field distinguishing credential references, plaintext configuration, and the pairing file. Linked external entries are not verified; orphaned references are shown explicitly. It does not enumerate unrelated vault entries, the external glab authentication store, or external `phab.cfg` files.

Add `--check` to read linked provider values only where a plaintext copy also exists. The comparison column reports `match`, `different`, or `unavailable`, without revealing either value, hashes or lengths. KeePassXC may request access approval. An unavailable vault leaves the comparison inconclusive. No values are changed, and matching values do not prove that the remote service still accepts the credential. `--check --json` includes the same results for scripts. Interactive tables use the CLI's shared colour styling; redirected output and JSON stay plain, and `NO_COLOR` disables table colours.

Add `--verify` to authenticate each stored copy separately. The `AUTH` column reports `valid`, `invalid`, `wrong-account`, `denied`, or an inconclusive `unavailable` result. For example, a current plaintext Phabricator token can show `valid` while the older linked token shows `invalid`. Checks support Gerrit, Phabricator, GitLab, Cloud VPS, and saved wiki accounts. Pairing files are not applicable; unknown credential types are unsupported. These are authenticated identity or token requests, not edits or an exhaustive permission check. Expired OAuth access tokens show `expired` without refreshing or rotating anything. No secrets are overwritten or removed. Supply keys to inspect only those credentials; `--check --verify` shares vault reads so each copy is retrieved once. JSON includes per-copy validation and the authenticated identity where available.

After `--check` or `--verify`, text output suggests commands when the results support a repair: remove an identical plaintext copy, replace an invalid vault copy with a valid plaintext copy, or log in again to Gerrit. Unverified differences get a verification command instead. Different account or endpoint bindings do not get replacement suggestions. Suggestions never run automatically; JSON remains the inventory array without command suggestions.

Existing plaintext credentials continue to work. Review and migrate supported entries explicitly:

```sh
mw config secrets migrate --dry-run
mw config secrets migrate
```

Migration verifies each saved value before removing its plaintext copy from configuration. This includes tokens already saved in CLI GitLab profiles. It does not import external `phab.cfg` files or upstream glab credentials; use the service login import options for those. Existing backups may still contain the old values.

When a plaintext value and a vault link both exist, ordinary migration refuses to change differing values. If you know the plaintext value is the one to keep, explicitly replace one named vault credential:

```sh
mw config secrets migrate phabricator.profiles.wikimedia.key --replace --dry-run
mw config secrets migrate phabricator.profiles.wikimedia.key --replace
```

This replaces the linked provider's value using plaintext from config, verifies the destination, then removes the plaintext copy. It does not verify that the token is accepted by the remote service. The saved account and endpoint must still match. `--replace` requires exactly one explicit key and a secure destination; it cannot replace multiple entries in a bulk migration.

### Gerrit account profiles

Gerrit and Phabricator expose the same `auth login`, `auth profiles` (`list` alias), `auth status`, `auth remove`, and `auth default` commands. Each accepts the service's persistent `--profile` flag; profile names have shell completion.

```sh
mw gerrit auth login --username addshore
mw gerrit auth login --username addbot --profile bot
mw gerrit auth profiles
mw gerrit auth status --profile bot
mw gerrit auth default --profile addshore
mw gerrit changes list --profile bot
mw gerrit ssh --profile bot version
mw gerrit auth remove --profile bot
```

Login validates the username with Gerrit's authenticated account endpoint. Without `--profile`, it uses that verified username in lowercase. The first login becomes the default; later logins ask before changing an existing default, with “no” selected initially. Non-interactive logins retain the existing default. `login --default` switches explicitly, and `--default=false` keeps the current default without prompting. You can also change it independently with `auth default --profile NAME`; Phabricator uses the same command.

Existing flat `gerrit.username` and `gerrit.password` configuration migrates automatically into a named profile, preserving credential references without retrieving secrets. Conflicting destination profiles abort migration without rewriting the file. The default profile's public username remains available to clone commands. SSH uses the selected profile's username and your SSH agent/config without retrieving its HTTP password.

`auth remove` deletes the selected local profile and credential or reference, and clears the default if that profile was selected. It does not revoke the HTTP password in Gerrit or delete an external KeePassXC entry. There is no Gerrit `auth logout` command. Listings, completion, and default changes do not open password managers or contact Gerrit.

### Phabricator login and account profiles

Browser login is the default. The CLI opens `/conduit/login/` in your browser (the Windows browser in WSL), where you can sign in with your Wikimedia or developer account. Paste the resulting token into the hidden terminal prompt. If KeePassXC is closed, the CLI attempts to launch it beside its configured proxy and waits up to two minutes for you to open and unlock it; connection attempts resume automatically. An unused command-line token initially expires after one hour; its first authenticated API request makes it permanent until revoked. The CLI verifies your identity before saving it in your selected credential provider.

```sh
mw phabricator auth login --profile personal
mw phabricator auth login --profile bot --method token
mw phabricator auth profiles
mw phabricator auth status --profile personal
mw phabricator view T12345 --profile bot
mw phabricator auth remove --profile bot
```

Use `--method token` to paste an existing API token without opening the browser. `--token-stdin` also skips browser launch and is suitable for automation. Add `--username Addbot` to reject a token belonging to another account. `--no-browser` prints the browser-login URL for you to open manually. `--url` explicitly selects a different HTTPS server; project-local `phab.cfg` never selects the destination for a newly entered token.

Profiles are independent saved account slots: personal and bot accounts can use the same server. Without `--profile`, login uses the verified username in lowercase (for example `addshore`); other commands use your saved default profile. The first login becomes the default. Later logins to another profile ask whether to change the default, with “no” selected initially. Non-interactive logins keep the existing default. Use `login --default` to switch explicitly, or `--default=false` to suppress the question and keep the current default. Use `--profile` on any Phabricator command to select another account. Set `phabricator.default_profile` to change the default independently; old CLI `sites` and `default_site` configuration is migrated automatically on upgrade, retaining profile names and saved credential references. The deprecated `--site` alias is hidden during the migration window. Profile names have shell completion; listing profiles does not retrieve tokens. `auth remove --profile NAME` removes the selected profile and its local credentials or reference, without revoking its server-side token. Revoke unwanted tokens in Phabricator's Conduit API Tokens settings.

Phabricator caches are isolated by server, profile, username and project, including
when profiles specify the same `cache_path`. A configured `cache_path` is treated
as a root for these separate caches. Older shared cache files are left in place
but are no longer read. `auth status` checks that the token belongs to the saved
username, allowing differences in letter case.

To copy credentials from the Python `phab` tool, explicitly import its configuration:

```sh
mw phab auth login --method phab-cfg --profile personal
mw phab auth login --method phab-cfg --config-file ~/.config/phab/phab.cfg --config-section bot --profile bot
```

The importer uses the specified section or the file's `[main] default`, verifies the configured identity on that section's HTTPS server, and saves its token in your chosen credential provider. It also copies `default_project`. The source file is left unchanged, including its plaintext token; the saved profile no longer depends on it. Normal commands never use credentials directly from `phab.cfg`. Detection in standard locations only suggests importing; it does not retrieve or test the token. Browser and existing-token login never take their destination from a detected file.

`config show` and `config get` redact secret fields by default. `--show-secrets` explicitly reveals plaintext configuration values; it does not retrieve keyring entries. Use a separate profile when changing accounts or endpoints, or remove the old profile with the service’s `auth remove --profile NAME` command before logging in again. Use `config secrets transfer` to change its storage backend. Profile removal deletes saved keyring credentials or KeePassXC references and local profile metadata; environment overrides are unchanged. Importing again requires an explicit login.

Choosing plaintext displays a warning that credentials are stored unencrypted. You can also select it with `--provider plaintext` or use `--insecure-storage` for a single save. A selected provider failing never silently switches storage. On Unix, the CLI restricts its config directory to the current user and writes the file atomically with mode `0600`. Windows file permissions depend on its ACLs; prefer Credential Manager there.

Configuration mutations share an interprocess lock and reload the current file before writing. OAuth refresh holds the same lock from reading the saved bundle through persisting rotated tokens, so concurrent commands reuse the new bundle instead of refreshing the same old token. Profile removal commits its credential and metadata changes together.

Plaintext migration also reads the source under the mutation lock. New native-keyring saves refuse to overwrite a different value in a slot not already linked by this configuration; an equal value can be linked safely.

The CLI's secure storage uses the reusable `pkg/secretstore` package. Provider access is separate from service authentication; migration from existing plaintext configuration remains an explicit operation.

GitLab supports the shared profile commands and secret providers:

```sh
mw gitlab auth login --profile addshore
mw gitlab auth login --profile addbot
mw gitlab auth profiles
mw gitlab auth default --profile addshore
mw gitlab auth status --profile addbot
mw gitlab mr list --profile addbot
mw gitlab auth remove --profile addbot
```

Login validates a personal access token with `/api/v4/user` before saving it. Without `--profile`, the verified username becomes the profile name. `--hostname` defaults to `gitlab.wikimedia.org`; HTTPS hosts without a subpath are supported. Choose the permissions you need when creating the token: `read_api` for read-only API operations, or `api` for API writes; Git over HTTPS also requires suitable repository access. Tokens keep their server-defined expiry; this implementation does not refresh OAuth sessions.

Use `--token-stdin` for piped input, or the hidden interactive prompt. `--provider` and `--insecure-storage` work as for other services. Profile tokens are stored under `gitlab.profiles.<name>.token` and appear in `config profiles` and `config secrets list --verify`.

To explicitly import an existing glab personal access token:

```sh
mw gitlab auth login --from-glab --profile addshore
```

Import validates and copies the saved token from glab configuration or its keyring. Environment tokens and OAuth sessions are not imported. The original glab credential is retained. Without a selected or default profile, existing GitLab operations retain upstream glab authentication. Once a profile is selected, it takes precedence over environment tokens and upstream credentials; a missing or locked profile never falls back to another account.

The embedded CLI receives the selected token in memory, without saving it in glab configuration. HTTP requests, redirects and pagination are restricted to the profile's HTTPS host. Git subprocesses receive a temporary host-scoped credential helper and the profile name, rather than the token in arguments or environment. An existing Git config file is not modified. The helper supplies the selected profile for HTTPS Git operations; SSH Git operations continue to use your SSH configuration and agent. `auth remove` removes local credentials or references without revoking the server token. External glab credentials remain managed separately.

## Saved wiki profiles

A wiki profile combines an identity, an authentication method, and one or more trusted sites. It belongs to a realm: Wikimedia is built in, while independent wikis and other farms use separate realms. The same username on different farms never shares credentials automatically. Wiki auth uses the same `login`, `profiles` (`list` alias), `status`, `default`, and `remove` command builders as Gerrit and Phabricator.

Use `--profile REALM/NAME`, or a short name with `--realm` or an explicitly selected wiki. Login without a name uses the verified username in lowercase (spaces become hyphens); supply a name explicitly for identities that need another spelling. For Wikimedia, the bundled CentralAuth site list supplies the full farm scope without configuring individual domains. This is a built-in allowlist, not runtime discovery from arbitrary wiki responses.

```sh
mw wiki auth login --realm wikimedia --profile addshore --provider keyring
mw wiki auth status --profile wikimedia/addshore
mw wiki page list --wiki commons.wikimedia.org --limit 5
```

OAuth opens a browser for authorization and uses PKCE without an embedded client secret. In WSL, links open in the Windows browser using `wslview` or PowerShell. Use `--no-browser` to open the displayed URL yourself. Access and refresh tokens are stored together in the selected credential provider. Configuration stores the account identity, default wiki, public client settings and credential reference. Expired access tokens refresh automatically; unavailable credentials or a failed refresh stop the command. Authorization may eventually need renewing if it expires or is revoked.

The default is the public **a-wikimedia-cli** OAuth 2 client, `b54c7ed6be6d2dc967a2d5101436f4a1`, registered with the exact `http://localhost:8765/auth/login` callback. Its registration is currently proposed: the proposer can test it while review is pending. Once approved, other users can authorize it too. No client secret is embedded or needed. Normal login listens for the browser callback and also accepts the complete callback URL through a hidden terminal prompt. If the Windows browser cannot reach the WSL listener, copy the URL from the browser address bar and paste it into the waiting terminal without restarting login. If port 8765 is occupied, the command automatically continues in paste-only mode.

To skip the listener from the start, use manual mode:
```sh
mw wiki auth login --realm wikimedia --profile addshore --provider keyring --manual --no-browser
```

Open the printed authorization URL, approve the application, then paste the complete resulting localhost URL into the terminal. An error page at localhost is expected if another service owns the port. The CLI validates the callback and state before exchanging the code with its private PKCE verifier. Callback input is hidden on terminals, and a pasted URL must match the state and redirect of the current login. The waiting input reader is cancelled when the browser callback succeeds. Do not put callback URLs or tokens in command arguments or logs.

To test another public client, supply both its client ID and its exact registered callback, for example `--client-id CLIENT_ID --redirect-uri http://localhost:3000/auth/mediawiki/callback`. Refresh can be checked explicitly without displaying tokens:

```sh
mw wiki auth status --profile wikimedia/addshore --refresh
```

Status reports each stage: reading the saved credential, refreshing with
Wikimedia, saving rotated tokens, and checking identity. An entry-update approval
in KeePassXC can keep the command waiting at the save stage. Wait for that
operation to finish before starting another forced refresh.

To recover an expired or invalid authorization, rerun `wiki auth login` with
`--replace`, the same `--profile`, and the intended client and scope options.
This explicitly updates the profile's linked credential entry after successful
authorization; it does not require logging out first. Changing the authentication
method requires a separate profile.

Interactive OAuth login uses one shared grant catalogue based on MediaWiki 1.46.2, with OAuth and Echo REL1_46 extension grants. The same list is shown for Wikimedia, wikibase.world and custom clients; it is not filtered by consumer registration. The wiki validates whether selected scopes are available to the registered consumer, so a selection can be rejected there.

The picker groups permissions by their upstream functional categories and labels every row with its risk rating (`low`, `vandalism`, `security`, `internal`, or `unknown`). Colours supplement those labels. Basic access is always included. Up/Down move, Space toggles an individual permission, typing filters the list, Escape clears the filter, and Enter confirms.

Right/Left move through cumulative global presets: Basic only → Low → Low + vandalism → Low + vandalism + security. Presets apply across the entire catalogue even when the list is filtered. Internal and unknown grants are never added or removed by these presets; they require individual selection and remain selected if explicitly chosen. An individual selection that differs from a preset is shown as Custom. Reauthorizing with the same client preserves previous requested scopes, including custom scopes absent from the catalogue, which appear under Unknown grants.

Use `--scope basic` to bypass the picker for basic-only access. Repeat `--scope` or use a comma-separated list such as `--scope basic,editpage`; values outside the bundled catalogue are also sent to the wiki for validation. Non-interactive login defaults to `basic`. `--all-scopes` explicitly requests the entire shared catalogue, including internal and unknown entries, as a saved list of scope names; it never omits the restriction to request an open-ended server default. `--scope` and `--all-scopes` cannot be combined. No dynamic discovery or grant cache is used.

The CLI saves the requested restriction with the profile and secure token bundle and sends it again during refresh. Reauthorize with `--replace` to change it. Use separate profiles to keep different permission levels separate.
`auth status` labels these as **requested** scopes, rather than claiming they
are server-verified grants. Verify restrictions through actual API operations:
a basic-only profile should authenticate successfully but fail an edit with
`permissiondenied`, including after refreshing its token. Wikimedia's OAuth
scopes resource lists the client's registered grants, not necessarily the
restricted token's effective permissions.

The built-in `wikibase-world` realm uses the public a-wikimedia-cli 1.1 client `8546b3108b91bd9ec7292c0aab4bb158`, with the same `http://localhost:8765/auth/login` callback:

```sh
mw wiki auth login --profile wikibase-world/addshore --scope basic,editpage
mw wiki auth status --profile wikibase-world/addshore
mw wiki auth status --profile wikibase-world/addshore --refresh
```

`--wiki https://wikibase.world/w/api.php` also selects this realm and client automatically. Authorization, exchange and refresh use wikibase.world's own `/w/rest.php/oauth2/` endpoints. The preset trusts only its exact API endpoint and cannot include Wikimedia or other sites. Tokens are stored under `wiki.profiles.wikibase-world.<name>.token` using the configured secret provider; the public client ID is metadata, not part of the secret key. Wikimedia profiles and credentials remain separate. All clients use the same shared permission catalogue; explicit `--scope` values and `--all-scopes` also work.

wikibase.world reported MediaWiki 1.43.11 when this client was added. Public-client refresh without a client secret was fixed in MediaWiki 1.46; older installations need a backport or upgrade. The CLI attempts refresh without a secret and preserves saved credentials if it fails, with instructions to reauthorize using `--replace`. Do not assume persistent login until the real refresh test succeeds.

Other MediaWiki sites can use a registered public client with explicit `--client-id`, `--redirect-uri`, `--wiki` and optionally `--realm`. Their OAuth endpoints are derived from the selected API's script path, including installations outside `/w`. The authority is saved with the profile; later refreshes never fall back to Wikimedia's token server.

For an arbitrary wiki, save its username/password using the same providers. Bot passwords such as `Addshore@cli` are also accepted. Supplying username/password options selects the password method unless `--method` was explicit:

```sh
mw wiki auth login --wiki https://wikibase.world/w/api.php --username Addshore --password-stdin
```

The realm is inferred from the endpoint (for example `wikibase-world`), and a new independent profile initially trusts only that endpoint. You can explicitly name the realm/profile:

```sh
mw wiki auth login --realm wikimedia --profile addbot --method password --username 'Addbot@cli' --password-stdin --provider keyring
mw wiki auth login --realm wikibase-world --profile addshore --wiki https://wikibase.world/w/api.php --method password --username 'Addshore@cli' --password-stdin --provider keyring
```

The first profile becomes the default. Later logins ask before switching the global default, with “no” selected initially; non-interactive logins preserve it. `--default` explicitly switches it, while `--default=false` avoids assigning either a global or realm default. Each realm also remembers a default profile for commands that explicitly select a wiki in that farm. `auth default --profile REALM/NAME` selects both that realm's default and the global default. The global setting is `wiki.default_profile`, and per-realm settings are `wiki.realms.REALM.default_profile`.

List profiles without unlocking a credential provider, select a default, or remove a local profile:
```sh
mw wiki auth profiles
mw wiki auth default --profile wikimedia/addbot
mw wiki auth remove --profile wikimedia/addshore
```

Shell completion suggests saved `--profile` names, `--realm` names, and
`--wiki` API endpoints. Account suggestions show public usernames and login
methods. Site suggestions use the bundled Wikimedia list and configured custom
realms; selecting a realm or account restricts the suggested sites accordingly.
Completion does not open credential stores or contact a wiki. For Bash, enable
completion in the current terminal with `source <(mw completion bash)`.

`auth remove` deletes the local profile and keyring credential or KeePassXC reference, clearing matching defaults. It does not revoke server-side OAuth authorization, change wiki passwords, or delete external KeePassXC entries. There is no wiki `auth logout` command.

Existing `wiki.accounts` and `default_account` settings migrate automatically to `wiki.profiles` and `default_profile`, preserving keyring locations and KeePassXC entry UUIDs without retrieving secrets. `--account` and `--user` remain hidden compatibility aliases for `--profile` and `--username`, with removal planned for early 2027.

Use repeated `--site API_URL` options at login to authorize an explicit set of sites for a profile. The login's `--wiki` endpoint is included. On Wikimedia this restricts the profile to the specified Wikimedia sites; it never authorizes unrelated sites. Reauthorization with `--replace` preserves an existing site restriction unless you explicitly supply new `--site` values.

```sh
mw wiki auth login --realm example-farm --profile personal --wiki https://one.example/w/api.php --site https://two.example/w/api.php --method password --username Example --password-stdin
```

Sites must actually accept the same credentials; adding an endpoint grants permission to use them there, it does not synchronize accounts. For a farm-wide shared scope, define the realm before login and omit `--site`:

```sh
mw wiki realm add example-farm --endpoint https://one.example/w/api.php --endpoint https://two.example/w/api.php
```

Credentials never cross realm boundaries based on matching usernames, similar hostnames, or redirects. On normal wiki commands, saved profiles cannot be combined with explicit `--username`, password flags, or `--anon`; omit the profile selection to use those one-off options. `--anon` bypasses saved accounts.

Wiki credentials participate in provider transfers and plaintext migration. Their keys are `wiki.profiles.REALM.NAME.token` for the OAuth token bundle and `wiki.profiles.REALM.NAME.password` for wiki passwords. For example:

```sh
mw config secrets transfer wiki.profiles.wikimedia.addshore.token --provider keepassxc
```

## Choosing storage

Select or change the default for new credentials and plaintext migrations:

```sh
mw config secrets provider keepassxc
mw config secrets provider
```

Available defaults are `keyring`, `keepassxc` and `plaintext`. Existing credentials keep their saved provider. Override the default for an individual login, configuration write or migration:

```sh
mw gerrit auth login --provider keyring
mw config set gerrit.profiles.addshore.password --stdin --provider keyring
mw config secrets migrate gerrit.profiles.addshore.password --provider keyring
```

To change an already linked credential’s provider, transfer it explicitly:

```sh
mw config secrets transfer gerrit.profiles.addshore.password --provider keyring
```

Transfer verifies and persists the destination before switching the reference, then removes the source if no current reference still uses it. KeePassXC requests GUI deletion approval. Use `--keep-source` to retain and track a backup copy; failed source cleanup keeps its record and leaves the verified destination active. A plaintext source is removed after verified secure storage unless `--keep-source` is set. Retained plaintext stays in config.json and is shown by `config secrets list`; it is not an external detached entry. It refuses to overwrite a different destination value. The default provider is unchanged. Pairing KeePassXC alone does not select it as the default. Discovery does not read credentials, unlock vaults or launch KeePassXC; selecting an unpaired KeePassXC installation starts its pairing flow.

For scripts and `--no-interaction`, set a default first or pass `--provider`. With no selection, noninteractive saves try the OS keyring and fail with guidance if it is unavailable; they never automatically choose plaintext.

The OS keyring is the simplest choice for credentials used on one computer: it integrates with that OS and needs no separate KeePassXC pairing. Availability and unlock policy depend on the desktop; WSL uses its Linux Secret Service, not Windows Credential Manager.

KeePassXC GUI access suits an existing shared vault, including Windows KeePassXC used from WSL. You control the database, backups and synchronization. It requires KeePassXC to be running, and adds a sensitive local pairing file. The CLI retrieves approved entries without handling the master password.

## Finding and cleaning up retained copies

Transfers remove unused source entries after destination verification by default. With `--keep-source`, or when cleanup fails, the original copy is retained. Retained transfers, KeePassXC relinking, and profile removal record the old provider's public identifiers in configuration; no credential values are stored in this record. List those retained entries with:

```sh
mw config secrets detached
mw config secrets detached --details
mw config secrets detached --json
```

Only entries not referenced by the current configuration are shown. Tracked entries are not checked for existence during listing. `--details` shows native target names or KeePassXC database and entry UUIDs, so they can also be located manually.

Use `--provider` to select a provider and discover older entries as well as its tracked copies:

```sh
mw config secrets detached --provider windows-credential-manager --details
mw config secrets detached --provider keepassxc --details
```

The Windows bridge restricts the native enumeration to `a-wikimedia-cli:` and the CLI's encoded `a-wikimedia-cli` secretstore namespace. Only metadata is returned across the WSL bridge. Known current profiles are matched to their older Windows slots where possible; others are marked unmapped. This scan does not modify the configuration or retrieve secret values into the Linux CLI.

Review the list, then explicitly select one ID to delete:

```sh
mw config secrets cleanup ID --yes
# Provider selection also rediscovers untracked entries when needed:
mw config secrets cleanup ID --provider windows-credential-manager --yes
mw config secrets cleanup ID --provider keepassxc --yes
```

The ID may be the 12-character value shown in the table or its full value from details/JSON. Cleanup rechecks current references under the credential mutation lock and refuses linked or ambiguous entries. Native deletion is verified before forgetting the retained record. KeePassXC verifies the database and UUID (and username when known), asks for GUI confirmation, and recycles the selected entry. Denial or failure keeps the retained record. No server-side token is revoked. Other configurations and applications may still use an unlinked entry; the scope is this configuration only.

Without `--provider`, listing uses only the metadata retention record. Selecting KeePassXC explicitly requests its newer, permission-gated `get-database-entries` API. This API returns database-wide titles, URLs and UUIDs, not passwords; the CLI filters results to `a-wikimedia-cli://` URLs and excludes active UUIDs. It does not enable the setting itself. In compatible KeePassXC versions, open Settings → Browser Integration → Advanced and enable “Allow limited access to all entries in connected database” (wording varies by version). This permits integrations to list metadata for every entry in connected databases. You can enable it temporarily, rerun discovery and cleanup, then turn it off again when finished. Unsupported versions or denied enumeration return this guidance in the error; ordinary tracked-entry listing and cleanup still work. Older untagged entries cannot be attributed to the CLI by this API and must be reviewed manually. Native Linux keyring discovery currently covers tracked entries only. Discovery never deletes anything.

## Native keyrings and Windows Credential Manager

Native credentials use the `a-wikimedia-cli` namespace. For example, Gerrit uses service `a-wikimedia-cli:gerrit:https://gerrit.wikimedia.org/r` and an account identifier combining the username and the credential-key hash. The configuration directory remains separate. Development entries in the abandoned `mwcli` credential namespace are not migrated or used as a fallback.

`keyring` uses the platform running the CLI: Linux Secret Service (also in WSL), macOS Keychain, or Windows Credential Manager. `windows-credential-manager` explicitly uses Windows storage, either natively or through Windows PowerShell launched from WSL. These providers never silently fall back to each other. Native storage follows the OS access rules; Windows generic credentials and Linux Secret Service do not provide KeePassXC-style per-entry application approvals.

```sh
mw config secrets status
mw config secrets status keyring
mw config secrets status keyring --check
```

Status reports availability without unlocking or retrieving saved credentials. `--check` explicitly writes, reads back, and deletes a uniquely named temporary test item; unlock your Linux keyring in the desktop password manager if prompted. If the default persistent collection is absent, create a default/login keyring using Passwords and Keys (Seahorse) or your desktop's password manager. A WSL Secret Service is independent of the Windows desktop.

For a temporary WSL test session when no session bus exists, use the documented [headless Linux keyring setup](https://keyring.readthedocs.io/en/stable/#using-keyring-on-headless-linux-systems). First supply a private runtime directory: some WSL sessions inherit `XDG_RUNTIME_DIR=/run/user/1000` even when that directory does not exist. Run the following in your own terminal and keep subsequent Linux-keyring commands in the new shell. The password prompt unlocks an existing login keyring, or creates one if missing; choose a password for a new keyring. The password is not echoed or placed in command history.

```sh
mwcli_keyring_runtime=$(mktemp -d /tmp/mwcli-keyring.XXXXXX)
env -u GNOME_KEYRING_CONTROL -u GNOME_KEYRING_PID \
  XDG_RUNTIME_DIR="$mwcli_keyring_runtime" \
  dbus-run-session -- bash --noprofile --norc
gnome-keyring-daemon --start --components=secrets
read -r -s -p 'Linux keyring password: ' mwcli_keyring_password
printf '\n'
printf '%s' "$mwcli_keyring_password" | gnome-keyring-daemon --unlock
unset mwcli_keyring_password
./bin/mw config secrets status keyring --check
```

The clean Bash shell prevents startup scripts from restoring stale session variables. This is a temporary D-Bus session, not a permanent WSL desktop-service setup. For persistent user-session infrastructure, see [Microsoft's WSL systemd setup](https://learn.microsoft.com/en-us/windows/wsl/systemd); GNOME Keyring still needs to be installed and unlocked. Stored credentials remain encrypted on disk, but accessing them later requires a running Secret Service and unlocking the same keyring.

For Windows storage from WSL, the CLI automatically launches the system `powershell.exe` with an embedded bridge script. No Windows CLI installation or additional PowerShell module is required. WSL interoperability must be enabled and Windows PowerShell must be available. Native Windows builds access Credential Manager directly.

```sh
mw config secrets status windows-credential-manager
mw config secrets status windows-credential-manager --check
mw config secrets transfer gerrit.profiles.addshore.password --provider windows-credential-manager
mw config secrets list gerrit.profiles.addshore.password --check --verify --details
```

The bridge uses bounded JSON over stdin/stdout, with secret bytes encoded in the pipe payload and no secrets in command arguments, logs, or temporary files. It accepts explicit Get/Set/Delete operations on CLI-owned service namespaces and a metadata-list operation restricted to CLI-owned target prefixes. Windows may supply credential blobs internally with enumeration; the bridge discards them and returns only names and account identifiers. These restrictions describe the bridge protocol; Windows Credential Manager itself follows the current user's logon-session permissions. PowerShell runs without loading user profiles or prompting. Credential requests are data passed through stdin, never executed as PowerShell code. Windows PowerShell policy and logon-session permissions still apply.

Transfer back with `--provider keyring` or `--provider keepassxc`. Every transfer reads back and verifies the destination before changing the reference, removes the unused source unless `--keep-source` is set, refuses a conflicting destination value, and leaves the default provider unchanged. `secrets list --verify` then checks service authentication using the newly active provider. Existing `keyring` references keep their original platform-specific meaning; use an explicit transfer to change stores.

Windows generic credentials are limited to 2,560 bytes, including UTF-8 encoding. Oversized OAuth token bundles are rejected without switching their reference; use KeePassXC for those credentials. No automatic truncation, splitting, or plaintext fallback is used.

## KeePassXC

Enable Browser Integration in KeePassXC and unlock the intended database. Pair the CLI, select it as the default, then preview and migrate existing plaintext credentials. Approve requests in KeePassXC:

```sh
mw config secrets keepassxc connect
mw config secrets provider keepassxc
mw config secrets migrate --dry-run
mw config secrets migrate
```

For Windows KeePassXC from WSL, put its installation directory on PATH or select its proxy once:

```sh
mw config secrets keepassxc connect --proxy '/mnt/c/Program Files/KeePassXC/keepassxc-proxy.exe'
```

KeePassXC handles database unlocking and entry approvals. Select the paired database before running a command. Credential operations request its unlock dialog and wait up to two minutes, bounded by the caller's deadline. Enable the Browser Integration setting allowing database unlock requests; otherwise unlock the database manually while the command waits. Timeout, denied access, a wrong database, or revoked pairing stops the operation. Credentials never fall back to plaintext or direct file access. KeePassXC may show entry-access dialogs according to its permissions settings.

New entries use the top-level `a-wikimedia-cli` group. Existing entries remain linked by database ID and entry UUID, so renaming the old `wm` group, moving an entry, or changing its title does not require re-linking. The entry's saved username and browser-access permissions must remain suitable. Deleting and recreating an entry changes its UUID: restore the original from KeePassXC's Recycle Bin or explicitly link the replacement with `config secrets use-keepassxc KEY --entry PATH`.

New credentials are created directly in the `a-wikimedia-cli` group. A credential-specific storage URL, such as `a-wikimedia-cli://phabricator.profiles.addshore.key`, makes KeePassXC use the credential key as its title. The real service endpoint stays in CLI configuration. Profiles with the same service and username therefore have separate entries, including separate OAuth grants or password credentials. Case-sensitive keys receive a distinguishing hash suffix because URL hostnames are case insensitive.

Existing linked entries retain their UUID and location, including older nested groups. You can move them into the flat group and rename them manually without re-linking. Conflicting unlinked entries are never overwritten. Each write is read back and verified before plaintext is removed. A dry run opens no GUI dialogs. KeePassXC records creation and modification timestamps itself; its browser API does not expose custom notes.

To link an entry you created yourself, use `mw config secrets use-keepassxc KEY --entry PATH`. Create it with the same username and password as the configured CLI account. `use-keepassxc KEY` reuses the entry path from an existing direct-access link; otherwise it uses `a-wikimedia-cli/KEY`. Use `--entry 'Wikimedia/Gerrit'` to select another path. The entry username must match the configured account. Linking verifies any retained plaintext before removing it, then pins the database ID and entry UUID so renaming the entry does not break access. Older direct-access links must be relinked with this command; their saved entry paths are reused.

Pairing credentials are stored unencrypted in `secrets/keepassxc-pairing.json` beside `config.json`, separate from normal configuration output. On Linux and WSL, the directory is private (`0700`) and the file is `0600`; Windows relies on the user profile's ACLs. Treat the pairing file as a secret: it grants access through KeePassXC while the database is unlocked, subject to its entry permissions. It contains neither your master password nor saved API passwords. One database pairing is supported at a time.

```sh
mw config secrets keepassxc status
mw config secrets keepassxc disconnect
```

`status` shows the pairing location and public metadata without contacting the GUI. `disconnect` removes the local pairing; also revoke its stored key in KeePassXC under **Database Settings → Browser Integration**. `auth remove --profile NAME` forgets an individual profile and its entry reference while retaining the shared pairing.

### Discovering saved accounts

List profiles across all supported services in one place:

```sh
mw config profiles
mw config profiles --details
mw config profiles --service wiki
mw config profiles --json
```

The summary shows service, profile, account, provider, and default status. Details add authentication methods, site scopes, and matching secret keys. `mw config profiles list` is also accepted. These commands read public metadata only and do not open credential providers or check authentication. External glab storage is managed separately.

Use `mw phab auth profiles`, `mw wiki auth profiles`, `mw gerrit auth profiles`, or `mw cloudvps auth profiles` to inspect saved account metadata without retrieving secrets or checking authentication. `auth list` remains an alias. Cloud VPS, Gerrit and Phabricator share the same profile commands and default-selection behavior.

To rotate a Cloud VPS application credential while keeping the same profile, run:

```sh
mw cloudvps auth login --profile mwcli --id NEW_APPLICATION_CREDENTIAL_ID --secret-stdin --replace
```

The CLI checks that the replacement authenticates to the profile's existing
project. It keeps the profile's provider and default selection, verifies and
persists the new credential, then removes the unused old local entry. KeePassXC
asks for deletion approval; if cleanup fails, the new credential remains active
and the old entry is tracked by `config secrets detached`. Failed validation or
storage leaves the old profile usable. OpenStack credentials are not revoked;
revoke the old application credential separately in Horizon. Use a separate
profile for a different project, and `config secrets transfer` to change providers.

Cloud VPS profiles each hold one application credential and its project. Login defaults the profile name to the project; use `--profile` to keep multiple credentials with different permissions for the same project:

```sh
mw cloudvps auth login --project mwcli --profile mwcli-readonly
mw cloudvps auth login --project mwcli --profile mwcli-admin
mw cloudvps auth profiles
mw cloudvps auth default --profile mwcli-readonly
mw cloudvps auth status --profile mwcli-readonly
mw cloudvps compute list --project mwcli --profile mwcli-readonly
mw cloudvps auth remove --profile mwcli-admin
```

The first saved profile becomes the default. Later logins ask before switching the default in an interactive terminal; use `--default` to select it explicitly. Resource commands use an explicit `--profile`, otherwise a matching default or the project's only saved profile. Multiple profiles without a matching default require `--profile`. An explicit `--profile` supplies its project unless `--project` is also given. Otherwise the project comes from `--project`, `cloud_vps.default_project`, or the default profile. A project/profile mismatch fails before credentials are read. SSH configuration and project selection remain separate from credentials.

Existing project-keyed credentials migrate to profiles named after their projects on startup. Migration preserves plaintext values and external references without opening a password manager. `auth add` and `auth check` remain aliases for `login` and `status`. New secret keys are `cloud_vps.profiles.<name>.secret`.
