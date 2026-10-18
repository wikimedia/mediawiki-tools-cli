# Toolforge shell and exec

Open an interactive SSH session or execute a command on Toolforge. Select a tool
account with `--tool`; commands then use that account's login environment and home
directory. Without `--tool`, use your normal Toolforge login account.

`shell` and its compatibility alias `exec` use the upstream Toolforge CLI. Put
`--` before a remote command with flags. Arguments, including spaces, quotes and
shell metacharacters, are preserved literally. To use a pipeline or redirection,
invoke `sh -c` explicitly. SSH and remote-command failures produce a nonzero exit
status.

Toolforge SSH access must already be configured. `--beta` selects toolsbeta;
`--lima` uses the local lima-kilo SSH configuration.

See [Toolforge documentation](https://wikitech.wikimedia.org/wiki/Help:Toolforge).
