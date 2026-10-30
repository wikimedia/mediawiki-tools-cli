# Credentials and password managers

On the first interactive Gerrit, Cloud VPS, Phabricator or wiki credential save, the CLI checks for an OS keyring and KeePassXC, then asks where to store credentials. Plaintext configuration is also an option. The choice becomes the default after a successful save. Configuration holds the account and a reference to the secret. Linux uses Secret Service over D-Bus, macOS uses Keychain, and Windows uses Credential Manager. Commands read credentials only when needed; unrelated commands do not unlock the keyring.

```sh
mw gerrit auth login
mw cloudvps auth add
mw phabricator auth login
```

Use hidden prompts or `--password-stdin`, `--secret-stdin` and `--token-stdin` rather than putting secrets in command arguments or shell history. A password-manager command can pipe its output into these flags. Do not enable shell tracing (`set -x`) around secrets.

## Existing credentials

Existing plaintext credentials continue to work. Review and migrate supported entries explicitly:

```sh
mw config secrets migrate --dry-run
mw config secrets migrate
```

Migration verifies each saved value before removing its plaintext copy from configuration. It does not import `phab.cfg` or migrate GitLab credentials. Existing backups may still contain the old values.

`config show` and `config get` redact secret fields by default. `--show-secrets` explicitly reveals plaintext configuration values; it does not retrieve keyring entries. Log out before changing a saved credential’s account or endpoint. Use `config secrets transfer` to change its storage backend. Logout removes saved keyring credentials or KeePassXC references; environment overrides are unchanged. Phabricator logout also disables legacy `phab.cfg` authentication for that site until the next login.

Choosing plaintext displays a warning that credentials are stored unencrypted. You can also select it with `--provider plaintext` or use `--insecure-storage` for a single save. A selected provider failing never silently switches storage. On Unix, the CLI restricts its config directory to the current user and writes the file atomically with mode `0600`. Windows file permissions depend on its ACLs; prefer Credential Manager there.

The CLI's secure storage uses the reusable `pkg/secretstore` package. Provider access is separate from service authentication; migration from existing plaintext configuration remains an explicit operation.

GitLab commands use the embedded GitLab CLI's separate authentication store. Use `mw gl auth login --use-keyring --stdin` for a new keyring login. This does not migrate or remove an existing token from GitLab's configuration file.

## Saved wiki accounts

Save an account once, then use it across wikis sharing the same login. Accounts belong to a realm: Wikimedia is built in, while independent wikis and other farms use separate realms. Identical usernames on different realms never share credentials automatically.

```sh
mw wiki auth login --realm wikimedia --account addshore --provider keyring
mw wiki auth status --account wikimedia/addshore
mw wiki page list --wiki commons.wikimedia.org --limit 5
```

OAuth opens a browser for authorization and uses PKCE without an embedded client secret. Access and refresh tokens are stored together in the selected credential provider. Configuration stores the account identity, default wiki, public client settings and credential reference. Expired access tokens refresh automatically; a locked provider or failed refresh stops the command. Authorization may eventually need renewing if it expires or is revoked.

The current default is Addshore's approved **basic-access development client**, not a production registration for a-wikimedia-cli. It is suitable for testing login, refresh, identity and reads, but not edits or uploads. Its exact callback is `http://localhost/auth/login` on port 80. If the port is occupied or unavailable, use manual mode:

```sh
mw wiki auth login --realm wikimedia --account addshore --provider keyring --manual --no-browser
```

Open the printed authorization URL, approve the application, then paste the complete resulting localhost URL into the terminal. An error page at localhost is expected if another service owns the port. The CLI validates the callback and state before exchanging the code with its private PKCE verifier. Callback input is hidden on terminals. Do not put callback URLs or tokens in command arguments or logs.

To test another public client, supply both its client ID and its exact registered callback, for example `--client-id CLIENT_ID --redirect-uri http://localhost:3000/auth/mediawiki/callback`. Refresh can be checked explicitly without displaying tokens:

```sh
mw wiki auth status --account wikimedia/addshore --refresh
```

For bots or wikis without OAuth, save a bot password using the same providers:

```sh
mw wiki auth login --realm wikimedia --account addbot --method bot-password --user 'Addbot@cli' --password-stdin --provider keyring
mw wiki auth login --realm wikibase-world --account addshore --wiki https://wikibase.world/w/api.php --method bot-password --user 'Addshore@cli' --password-stdin --provider keyring
```

Each successful login makes that account its realm's default, and makes that realm the default when no wiki is supplied. The stored wiki is used when `--wiki` is omitted. Use `--account REALM/NAME` to select another account explicitly. Account names are stable configuration identifiers containing letters, digits, underscores or hyphens. List identities without unlocking any credential provider, or change a default:

```sh
mw wiki auth list
mw wiki auth default --account wikimedia/addbot
mw wiki auth logout --account wikimedia/addshore
```

Logout removes the local account and keyring credential, or forgets its KeePassXC reference. It does not revoke server-side OAuth authorization or delete external KeePassXC entries.

Wikimedia realm membership comes from a bundled list of exact CentralAuth API endpoints, so normal use needs no domain configuration or discovery request. An unknown custom realm initially trusts only the endpoint used at login. For a shared-login farm, explicitly define its trusted endpoints before saving an account:

```sh
mw wiki realm add example-farm --endpoint https://one.example/w/api.php --endpoint https://two.example/w/api.php
```

Credentials never cross realm boundaries based on matching usernames, similar hostnames, or redirects. Saved accounts cannot be combined with explicit `--user`, password flags, or `--anon`; omit the account selection to use those one-off options. `--anon` bypasses saved accounts.

Wiki credentials participate in provider transfers and plaintext migration. Their keys are `wiki.accounts.REALM.NAME.token` for the OAuth token bundle and `wiki.accounts.REALM.NAME.password` for bot passwords. For example:

```sh
mw config secrets transfer wiki.accounts.wikimedia.addshore.token --provider keepassxc
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
mw config set gerrit.password --stdin --provider keyring
mw config secrets migrate gerrit.password --provider keyring
```

To change an already linked credential’s provider without logging out, transfer it explicitly:

```sh
mw config secrets transfer gerrit.password --provider keyring
```

Transfer verifies the destination before switching the reference. External source entries are retained; a plaintext config copy is removed after verified secure storage. It refuses to overwrite a different destination value. The default provider is unchanged. Pairing KeePassXC alone does not select it as the default. Discovery does not read credentials, unlock vaults or launch KeePassXC; selecting an unpaired KeePassXC installation starts its pairing flow.

For scripts and `--no-interaction`, set a default first or pass `--provider`. With no selection, noninteractive saves try the OS keyring and fail with guidance if it is unavailable; they never automatically choose plaintext.

The OS keyring is the simplest choice for credentials used on one computer: it integrates with that OS and needs no separate KeePassXC pairing. Availability and unlock policy depend on the desktop; WSL uses its Linux Secret Service, not Windows Credential Manager.

KeePassXC GUI access suits an existing shared vault, including Windows KeePassXC used from WSL. You control the database, backups and synchronization. It requires KeePassXC to be running with the paired database unlocked, and adds a sensitive local pairing file. The CLI retrieves approved entries without handling the master password.

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

KeePassXC handles database unlocking and entry approvals. Select the paired database before running a command. A locked database, denied request or revoked pairing fails without falling back to plaintext or direct file access. KeePassXC may show entry-access dialogs according to its permissions settings.

New credentials are created directly in the `wm` group, with the service hostname as the entry title and the account as the username. Multiple accounts can share a hostname. KeePassXC’s browser protocol cannot set a custom title; after linking, you can rename entries to labels such as “Gerrit — addshore” or “Cloud VPS — mwcli”, or move them, without breaking their UUID references. Existing linked entries are updated through the GUI. Each write is read back and verified before plaintext is removed. A dry run opens no GUI dialogs.

To link an entry you created yourself, use `mw config secrets use-keepassxc KEY --entry PATH`. Create it with the same username and password as the configured CLI account. `use-keepassxc KEY` reuses the entry path from an existing direct-access link; otherwise it uses `wm/KEY`. Use `--entry 'Wikimedia/Gerrit'` to select another path. The entry username must match the configured account. Linking verifies any retained plaintext before removing it, then pins the database ID and entry UUID so renaming the entry does not break access. Older direct-access links must be relinked with this command; their saved entry paths are reused.

Pairing credentials are stored unencrypted in `secrets/keepassxc-pairing.json` beside `config.json`, separate from normal configuration output. On Linux and WSL, the directory is private (`0700`) and the file is `0600`; Windows relies on the user profile's ACLs. Treat the pairing file as a secret: it grants access through KeePassXC while the database is unlocked, subject to its entry permissions. It contains neither your master password nor saved API passwords. One database pairing is supported at a time.

```sh
mw config secrets keepassxc status
mw config secrets keepassxc disconnect
```

`status` shows the pairing location and public metadata without contacting the GUI. `disconnect` removes the local pairing; also revoke its stored key in KeePassXC under **Database Settings → Browser Integration**. Logout forgets an individual entry reference while retaining the shared pairing.
