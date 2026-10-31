# Wikimedia Gerrit

Save HTTP credentials with `mw gerrit auth login --username YOURNAME`.
Use `auth profiles` to list accounts, `auth default --profile NAME` to choose a
default, and `--profile NAME` on any Gerrit command to select another account.
`auth remove --profile NAME` removes local credentials without revoking them.

SSH commands use the selected profile's username and your configured SSH keys.
Your SSH agent/config must support connecting to `gerrit.wikimedia.org`.

## Documentation

- [Gerrit SSH Access](https://www.mediawiki.org/wiki/Gerrit/Tutorial#Set_Up_SSH_Keys_in_Gerrit)
