<!-- Imported from https://www.mediawiki.org/wiki/Cli/guide/Docker-Development-Environment/MediaWiki?oldid=8308003 (CC BY-SA 4.0); see the page history for contributors. -->

# Work with MediaWiki

## Remote development

Run the environment on an SSH-accessible Linux VM. New contexts receive a two-word name and keep named checkouts under `~/wm/CONTEXT/`, with private CLI state under `.wm/`:

```sh
mw dev remote open --ssh awm-1.boxd --recipe mediawiki
# Or discover a Cloud VPS instance using local credentials:
mw dev remote open --project mwcli --instance <instance-id> --recipe mediawiki
```

The session runs in the background, maintaining SSH tunnels and reconnecting when necessary. New contexts start without an editor. Use `--editor code` for desktop VS Code and `--editor web` to run [code-server](https://coder.com/docs/code-server) on the VM through an authenticated SSH tunnel. Reopening a context uses its saved editor choice. `--editor none` clears that choice. Wiki URLs are printed by default. Use `--open-browser` to open the wiki when it is ready. The web editor opens its own tab when selected. Use `--no-browser` to suppress automatic tabs. Linux, macOS, native Windows and WSL clients are supported; WSL can launch Windows VS Code.

```sh
mw dev remote list
mw dev remote open calm-otter
mw dev --context calm-otter dc ps
mw dev remote disconnect calm-otter
```

Disconnect retains remote services and files. To forget a context, use `mw dev remote remove CONTEXT`. Add `--delete-workspace` to destroy its containers and volumes and delete its remote source, including uncommitted work; deletion requires confirmation or `--yes`. Local source directories and the VM are retained. Failed remote teardown keeps the registration so cleanup can be retried.

Use `mw dev remote cache cleanup --dry-run` to preview unused local release downloads. Add a context name or `--ssh HOST` to include that VM’s shared CLI and browser-editor downloads. Remove `--dry-run` to confirm deletion; referenced or running downloads are retained.

Local `mw dev` commands targeting a registered remote context run on its VM, with stdin, output and exit status preserved. Disconnecting removes session access while retaining remote files and containers. Reopening does not update source or reinstall the wiki. Existing local contexts keep running locally.

The VM needs Git and Docker Compose with permission to use Docker. `--install-deps` installs dependencies using passwordless sudo on supported Debian/Ubuntu systems. Stable clients download and checksum-verify the exact same release for the VM’s Linux architecture, then cache and upload it. Development and nightly clients only copy their running executable and require a matching Linux OS and architecture; they never download a different build. A missing matching release stops setup. The `mediawiki` recipe installs core, Vector and MariaDB; `--recipe none` prepares a workspace without installing an application.

### Create or connect to a provider machine

```sh
wm dev remote create sandbox --provider lima --recipe mediawiki
wm dev remote create farm --provider codespaces --recipe wmf-wikifarm --idle-timeout 30m
wm dev remote create wiki --provider boxd --recipe mediawiki
wm dev remote create cloud-wiki --provider cloudvps --project PROJECT --recipe mediawiki
wm dev remote open existing --provider cloudvps --project PROJECT --target INSTANCE_ID
wm dev remote open farm
wm dev --context farm compose ps
```

Lima needs `limactl`. Codespaces and Boxd use a signed-in `gh` or `boxd` on
the client. Cloud VPS uses the CLI's Cloud VPS credentials and SSH settings.
Cloud VPS requires `--project`. It resolves the current IDs for Debian Trixie,
`g4.cores2.ram4.disk20`, and `VXLAN/IPv6-dualstack` by name. Use `--image`,
`--flavor`, or `--network` to override them by ID. Use `--key-name` for an
image that needs an injected SSH key. `--user-data FILE` supplies optional
cloud-init configuration.

Boxd uses the organisation selected in the Boxd CLI. Codespaces (also
spelled `github` in `--provider`) defaults to `github/codespaces-blank` with a
32 GB machine, or a 64 GB machine for `wmf-wikifarm`. Use `--repo` and
`--machine` to override those settings. GitHub applies the chosen
repository’s billing policy.

The gateway port is chosen automatically from ports free on both the local machine and the VM. Use `--port` to choose a specific port on first setup. Reopening reuses the saved port.

Provider workspaces start without a recipe or credential forwarding.
Add `--recipe` on first setup if you want an application installed. New
machines created by the command can install missing dependencies on supported
Debian or Ubuntu images using passwordless sudo. Use `--install-deps=false`
to check dependencies only. Existing machines require an explicit
`--install-deps` to make system changes.

```sh
wm dev remote list --all --running
wm dev remote list --all --provider cloudvps --project PROJECT
wm dev remote disconnect farm
wm dev remote stop farm
wm dev remote destroy farm
```

The list shows provider machine status separately from the SSH session.
`disconnect` closes the session. `stop` leaves the disk in place, and
`destroy` permanently deletes it after confirmation. Machine lifecycle
commands are available for machines created by `wm`.

The wiki farm uses local HTTPS port `8443`. The CLI prints the forwarded
wiki and dashboard URLs and the path to the farm's public HTTPS certificate.
Trust that certificate in your development browser to use the HTTPS sites.

To edit an existing local project while running it remotely:

```sh
mw dev remote open --ssh awm-1.boxd --directory . --source-name citoid --recipe none --editor code
mw dev remote sync calm-otter
```

This opens the local editor and continuously synchronizes files to the VM using rsync, which must be installed on both machines. On Windows, WSL is the simplest option for this mode. Git metadata, dotenv files, dependency directories and links outside the project are excluded. Remote overwrites/deletions are backed up under `.wm/sync-backup`; this is one-way synchronization, so keep edits in the local checkout. Local `mw dev` commands flush pending changes before executing remotely.

Phabricator task, project, board and paste commands can run through the active session bridge, using profiles and credential stores on the local machine. Input files are transferred as stdin. Credential-management commands, shells and reads of local files are not delegated. `--forward none` disables delegation. Once disconnected, remote commands cannot use the bridge.

For a private checkout or authenticated Git operations, select the destination locally with `--git-remote <URL>`. The CLI clones/fetches/pushes using a private local mirror and transfers Git objects over SSH. The VM receives source objects, not local Git configuration or service credentials. In the remote editor's terminal, `git fetch`, `git push` and fast-forward-only `git pull` use that destination; ordinary Git commands stay on the VM. Local Git authentication is reused, including the saved CLI profile for Wikimedia GitLab HTTPS. Other Git destinations use normal local Git/SSH configuration.

You can get a shell as your current user in the main MediaWiki container easily:

```
mw dev mediawiki exec bash
```

You can use the exec command to perform actions inside the MediaWiki container as a one off...

Such as, watching logs

```
mw dev mediawiki exec -- tail -f /var/log/mediawiki/debug.log
```

## Composer download failures

Automatic dependency setup during installation, recipes and `update-code --composer`
retries explicit DNS, connection and download-timeout failures up to three attempts,
waiting 2 seconds and then 5 seconds between attempts. Output stays visible, and
messages show when another attempt is starting. Composer may also retry individual
requests internally.

Dependency conflicts, invalid configuration and script failures still stop the
operation. A failed Composer update stops installation before the database stage.
If network failures persist, check DNS and connectivity inside the MediaWiki
container before rerunning the command. The explicit `mw dev mediawiki composer`
command passes through to Composer without this additional retry policy.

## PHPUnit tests

You can run PHPUnit tests in the MediaWiki container easily:

```
mw dev mediawiki exec -- composer phpunit tests/phpunit/unit/includes/XmlTest.php
```

You may still need to use the old phpunit.php test running for some PHPUnit tests!

```
mw dev mediawiki exec -- php tests/phpunit/phpunit.php tests/phpunit/unit/includes/xml/XmlTest.php
```

If you want to run tests with the debugger, see the XDebug section below.

## XDebug

See [debug PHP with Xdebug](xdebug.md) for IDE setup, triggers, container settings and troubleshooting. Use [Fresh](fresh.md) to run browser tests and record Selenium videos.

## See also

- [User:Santhosh.thottingal/WikiFamily](https://www.mediawiki.org/wiki/User:Santhosh.thottingal/WikiFamily) - Multi wiki setup and how to add new services to the development environment
