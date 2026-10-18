# Credentials and password managers

Gerrit, Cloud VPS and Phabricator logins save secrets in the OS keyring. Configuration holds the account and a reference to the secret. Linux uses Secret Service over D-Bus, macOS uses Keychain, and Windows uses Credential Manager. Commands read credentials only when needed; unrelated commands do not unlock the keyring.

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

`config show` and `config get` redact secret fields by default. `--show-secrets` explicitly reveals plaintext configuration values; it does not retrieve keyring entries. Log out before changing a saved credential to another account, endpoint or storage backend. Logout removes saved keyring credentials; environment overrides are unchanged. Phabricator logout also disables legacy `phab.cfg` authentication for that site until the next login.

If the selected provider is unavailable, login fails rather than silently saving plaintext. Add `--insecure-storage` to explicitly use the config file instead. On Unix, the CLI restricts its config directory to the current user and writes the file atomically with mode `0600`. Windows file permissions depend on its ACLs; prefer Credential Manager there.

GitLab commands use the embedded GitLab CLI's separate authentication store. Use `mw gl auth login --use-keyring --stdin` for a new keyring login. This does not migrate or remove an existing token from GitLab's configuration file.

## KeePassXC

Enable Browser Integration in KeePassXC and unlock the intended database. Pair the CLI, then link an existing entry. Approve requests in KeePassXC:

```sh
mw config secrets keepassxc connect
mw config secrets use-keepassxc gerrit.password
```

For Windows KeePassXC from WSL, put its installation directory on PATH or select its proxy once:

```sh
mw config secrets keepassxc connect --proxy '/mnt/c/Program Files/KeePassXC/keepassxc-proxy.exe'
```

KeePassXC handles database unlocking and entry approvals. Select the paired database before running a command. A locked database, denied request or revoked pairing fails without falling back to plaintext or direct file access. KeePassXC may show entry-access dialogs according to its permissions settings.


To link an entry you created yourself, use `mw config secrets use-keepassxc KEY --entry PATH`. Create it with the same username and password as the configured CLI account. `use-keepassxc KEY` reuses the entry path from an existing direct-access link; otherwise it uses `wm/KEY`. Use `--entry 'Wikimedia/Gerrit'` to select another path. The entry username must match the configured account. Linking verifies any retained plaintext before removing it, then pins the database ID and entry UUID so renaming the entry does not break access. Older direct-access links must be relinked with this command; their saved entry paths are reused.

Pairing credentials are stored unencrypted in `secrets/keepassxc-pairing.json` beside `config.json`, separate from normal configuration output. On Linux and WSL, the directory is private (`0700`) and the file is `0600`; Windows relies on the user profile's ACLs. Treat the pairing file as a secret: it grants access through KeePassXC while the database is unlocked, subject to its entry permissions. It contains neither your master password nor saved API passwords. One database pairing is supported at a time.

```sh
mw config secrets keepassxc status
mw config secrets keepassxc disconnect
```

`status` shows the pairing location and public metadata without contacting the GUI. `disconnect` removes the local pairing; also revoke its stored key in KeePassXC under **Database Settings → Browser Integration**. Logout forgets an individual entry reference while retaining the shared pairing.
