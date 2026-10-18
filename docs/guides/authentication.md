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
