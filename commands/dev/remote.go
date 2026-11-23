package dev

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/cloudvps"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devremote"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func NewRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "remote", Short: "Manage remote development contexts and background sessions"}
	var opts devremote.Options
	var host, project, instance, user, jump, directory, sourceName, gitRemote, lima string
	var forwarded []string
	var provider providerOptions
	open := &cobra.Command{
		Use: "open [context]", Short: "Prepare or reconnect a background remote development session",
		Long: `Keep code and containers on a Linux VM, with browser access maintained by a background SSH session.
Use --provider with --target to open an existing Lima, Codespaces, Boxd or Cloud VPS machine. Use remote create to provision one. Provider workspaces start with no recipe or credential delegation unless requested.
New contexts receive a two-word name and use ~/wm/CONTEXT/mediawiki by default. Reopen a saved context without supplying its SSH target again.
Use --directory for continuous local-to-remote source synchronization. Git metadata, dotenv files and dependency directories are excluded; overwritten remote files are retained under .wm/sync-backup. Local-folder mode defaults to no recipe. With --editor code, the local folder opens in VS Code.
New contexts start without an editor. Choose --editor code for desktop VS Code or --editor web for an editor on the VM accessible through an authenticated SSH tunnel. Wiki links are printed by default. Use --open-browser to open the wiki.
Stable releases download the exact release’s Linux build for the VM. Development and nightly builds copy this executable and require matching Linux OS/architecture. --install-deps installs Git and Docker on supported Debian/Ubuntu VMs using passwordless sudo.
Phabricator commands can execute on this machine through the active session bridge (--forward phab, the default for direct SSH workspaces). Credential management and shell commands are not delegated; file arguments are transferred as stdin. --forward none disables delegation.
--git-remote selects an authenticated Git destination on this machine. Fetch/push transfer objects through a local mirror; no service credentials or SSH agents are copied to the VM.`,
		Example: cobrautil.NormalizeExample(`open --ssh awm-1.boxd --recipe mediawiki
open sandbox --provider lima --target my-vm
open --project mwcli --instance ea86a663-eee4-4ed7-9962-d7f339800b5c --editor web
open calm-otter
open --ssh my-vm --directory . --recipe none --editor code`),
		Args: cobra.MaximumNArgs(1), PersistentPreRunE: cobrautil.CallAllPersistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := devenv.Context
			explicit := cmd.Flags().Changed("context") || os.Getenv("CONTEXT") != ""
			if len(args) > 0 {
				if explicit && name != args[0] {
					return fmt.Errorf("use either a context argument or a matching --context")
				}
				name = args[0]
				explicit = true
			}
			if lima != "" && (host != "" || instance != "" || project != "" || user != "" || jump != "") {
				return fmt.Errorf("--lima cannot be combined with SSH or Cloud VPS target options")
			}
			creating := cmd.Name() == "create"
			provider.kind = canonicalRemoteProvider(provider.kind)
			if err := provider.validate(cmd, creating, project, host, instance, lima, user, jump); err != nil {
				return err
			}
			target := host != "" || instance != "" || lima != "" || provider.kind != ""
			if !explicit && target {
				var err error
				name, err = devremote.NewContextName()
				if err != nil {
					return err
				}
			}
			if !explicit && !target {
				contexts, err := devremote.Contexts()
				if err != nil {
					return err
				}
				if len(contexts) != 1 {
					return fmt.Errorf("specify a context; use remote list to see saved contexts")
				}
				name = contexts[0].Context
			}
			unlock, err := devremote.LockProvider(cmd.Context(), name)
			if err != nil {
				return err
			}
			defer unlock()
			c, err := devremote.LoadContext(name)
			fresh := os.IsNotExist(err)
			if err != nil && !fresh {
				return err
			}
			if creating && !fresh {
				return fmt.Errorf("context %s already exists; use remote open %s", name, name)
			}
			if fresh {
				if _, err := os.Stat(filepath.Join(cli.UserDirectoryPathForCmd("mwdd"), name)); err == nil {
					return fmt.Errorf("context %q already belongs to a local environment; choose another remote context name", name)
				} else if !os.IsNotExist(err) {
					return err
				}
				c = devremote.Context{Options: opts, SourceName: "mediawiki", Forward: []string{"phab"}}
				c.Context = name
			}
			if c.Provider != nil {
				if target && (provider.kind != c.Provider.Kind || provider.target != c.Provider.ID || project != c.Provider.Project) {
					return fmt.Errorf("this context already belongs to another target; choose a new context name")
				}
			} else if provider.kind != "" {
				if !fresh {
					return fmt.Errorf("this context already belongs to another target; choose a new context name")
				}
				c.Provider = &devremote.ProviderTarget{Kind: provider.kind, ID: provider.target, Project: project, User: user, Jump: jump}
			} else if lima != "" || c.Lima != "" {
				if !fresh && (host != "" || instance != "" || (lima != "" && c.Lima != lima)) {
					return fmt.Errorf("this context already belongs to another target; create another context")
				}
				if fresh {
					c.Lima = lima
				}
			} else if target {
				if host != "" && instance != "" {
					return fmt.Errorf("provide either --ssh or --project with --instance")
				}
				var sshArgs []string
				if host != "" {
					if project != "" || user != "" || jump != "" {
						return fmt.Errorf("with --ssh, configure the user and jump host in SSH configuration")
					}
					if err := devremote.ValidateHost(host); err != nil {
						return err
					}
					sshArgs = []string{host}
				} else {
					if project == "" {
						return fmt.Errorf("--instance requires --project")
					}
					sshArgs, err = cloudvps.RemoteSSHArgs(cmd, project, instance, user, jump)
					if err != nil {
						return err
					}
					clean := sshArgs[:0]
					for _, arg := range sshArgs {
						if arg != "-T" {
							clean = append(clean, arg)
						}
					}
					sshArgs = clean
				}
				if !fresh && !reflect.DeepEqual(sshArgs, c.SSHArgs) {
					return fmt.Errorf("this context already belongs to another SSH target; create another context")
				}
				c.SSHArgs = sshArgs
			} else if fresh {
				return fmt.Errorf("a new context needs --lima, --ssh or --project with --instance")
			}
			previousEditor := c.Editor
			previousGit := c.GitRemote
			if !fresh && cmd.Flags().Changed("recipe") && c.Recipe != opts.Recipe {
				return fmt.Errorf("use another context for another recipe")
			}
			if fresh || cmd.Flags().Changed("recipe") {
				c.Recipe = opts.Recipe
				if !cmd.Flags().Changed("recipe") {
					c.Recipe = "mediawiki"
					if c.Provider != nil {
						c.Recipe = "none"
					}
				}
			}
			if fresh || cmd.Flags().Changed("editor") {
				c.Editor = opts.Editor
			}
			c.NoBrowser = opts.NoBrowser
			c.OpenBrowser = opts.OpenBrowser
			if fresh || cmd.Flags().Changed("port") {
				c.Port = opts.Port
			}
			if cmd.Flags().Changed("directory") {
				resolved, err := devremote.LocalDirectory(directory)
				if err != nil {
					return err
				}
				if !fresh && c.Directory != resolved {
					return fmt.Errorf("use another context for another source directory")
				}
				c.Directory = resolved
				if !cmd.Flags().Changed("recipe") && fresh {
					c.Recipe = "none"
				}
				if !cmd.Flags().Changed("source-name") && c.Recipe == "none" {
					c.SourceName = filepath.Base(resolved)
				}
			}
			if cmd.Flags().Changed("source-name") {
				if !fresh && c.SourceName != sourceName {
					return fmt.Errorf("use another context for another source name")
				}
				c.SourceName = sourceName
			}
			if cmd.Flags().Changed("forward") {
				c.Forward = nil
				for _, service := range forwarded {
					if service == "none" {
						continue
					}
					if service != "phab" {
						return fmt.Errorf("supported delegated service: phab")
					}
					c.Forward = append(c.Forward, service)
				}
			} else if fresh && c.Provider != nil {
				c.Forward = nil
			}
			if cmd.Flags().Changed("git-remote") {
				if err := devremote.ValidateGitRemote(gitRemote); err != nil {
					return err
				}
				if !fresh && previousGit != "" && gitRemote != "" && gitRemote != previousGit {
					return fmt.Errorf("use another context for another Git destination")
				}
				c.GitRemote = gitRemote
			}
			if cmd.Flags().Changed("install-deps") || fresh {
				c.InstallDeps = opts.InstallDeps || (creating && !cmd.Flags().Changed("install-deps"))
			}
			if _, err := osexec.LookPath("ssh"); err != nil {
				return fmt.Errorf("install OpenSSH on this computer: %w", err)
			}
			// Switching to/from a web editor changes the session's forwarding set.
			if !fresh && ((previousEditor != c.Editor && (previousEditor == "web" || c.Editor == "web")) || previousGit != c.GitRemote) {
				_ = devremote.Disconnect(cmd.Context(), name)
				for i := 0; i < 50; i++ {
					status, _ := devremote.SessionStatus(cmd.Context(), name)
					if strings.Contains(status, ": disconnected") {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			if c.Provider != nil {
				if err := c.Options.ValidateSettings(); err != nil {
					return err
				}
				if creating {
					provisioned, err := provider.provision(cmd, name, project, user, jump, c.Recipe)
					if err != nil {
						return err
					}
					c.Provider = &provisioned
					cmd.Printf("Created %s machine %s for context %s.\n", provisioned.Kind, provisioned.ID, name)
					if err := devremote.SaveContext(c); err != nil {
						return fmt.Errorf("save context for created machine %s: %w", provisioned.ID, err)
					}
				}
				if fresh || devremote.ConnectionState(cmd.Context(), name) != "connected" {
					c.SSHArgs, err = providerSSH(cmd, c)
					if err != nil {
						return err
					}
				}
			}
			if c.Lima != "" {
				if err := c.Options.ValidateSettings(); err != nil {
					return err
				}
				var err error
				c.SSHArgs, err = devremote.LimaSSH(cmd.Context(), c.Lima, fresh, cmd.OutOrStdout(), cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}
			return devremote.StartSession(cmd.Context(), c, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	open.Flags().StringVar(&lima, "lima", "", "Start or create a named Lima Docker VM")
	open.Flags().StringVar(&host, "ssh", "", "SSH host or configured alias")
	open.Flags().StringVar(&project, "project", "", "Cloud VPS project for provisioning or attaching")
	open.Flags().StringVar(&instance, "instance", "", "Cloud VPS instance ID")
	open.Flags().StringVar(&user, "user", "", "Cloud VPS SSH username")
	open.Flags().StringVar(&jump, "jump-host", "", "Cloud VPS SSH bastion override")
	open.Flags().StringVar(&opts.Recipe, "recipe", "", "Built-in recipe or none (new direct connections default to mediawiki; provider and local-folder workspaces default to none)")
	open.Flags().IntVar(&opts.Port, "port", 0, "Environment port (first setup only; automatically selected by default)")
	open.Flags().StringVar(&opts.Editor, "editor", "none", "Editor to open (code, web or none)")
	open.Flags().BoolVar(&opts.OpenBrowser, "open-browser", false, "Open the wiki URL in your browser when ready")
	open.Flags().BoolVar(&opts.NoBrowser, "no-browser", false, "Suppress automatic browser tabs, including the web editor")
	open.Flags().BoolVar(&opts.InstallDeps, "install-deps", false, "Install remote dependencies using passwordless sudo")
	open.Flags().StringVar(&directory, "directory", "", "Continuously synchronize this local project directory to the VM")
	open.Flags().StringVar(&sourceName, "source-name", "mediawiki", "Name of the codebase directory within the context")
	open.Flags().StringSliceVar(&forwarded, "forward", nil, "Locally executed services: phab by default for direct connections, none for new provider workspaces")
	open.Flags().StringVar(&gitRemote, "git-remote", "", "Git destination authenticated locally (enables the Git bridge)")
	provider.addFlags(open)
	create := &cobra.Command{
		Use: "create CONTEXT", Short: "Create a provider machine and open a remote workspace",
		Example: cobrautil.NormalizeExample("create wiki --provider cloudvps --project mwcli --recipe mediawiki\ncreate wiki --provider codespaces --recipe mediawiki\ncreate wiki --provider boxd --recipe mediawiki\ncreate wiki --provider lima --recipe mediawiki"),
		Args:    cobra.ExactArgs(1), PersistentPreRunE: cobrautil.CallAllPersistentPreRunE, RunE: open.RunE,
	}
	create.Flags().AddFlagSet(open.Flags())
	cmd.AddCommand(open, create)
	var all, running bool
	var inventoryProvider, inventoryProject string
	status := &cobra.Command{Use: "status [context]", Aliases: []string{"list", "contexts"}, Short: "List remote contexts, sessions and provider machine states", Args: cobra.MaximumNArgs(1), PersistentPreRunE: cobrautil.CallAllPersistentPreRunE, RunE: func(cmd *cobra.Command, args []string) error {
		contexts, err := devremote.Contexts()
		if err != nil {
			return err
		}
		if len(args) > 0 {
			c, err := devremote.LoadContext(args[0])
			if err != nil {
				return err
			}
			contexts = []devremote.Context{c}
		} else if cmd.Flags().Changed("context") {
			c, err := devremote.LoadContext(devenv.Context)
			if err != nil {
				return err
			}
			contexts = []devremote.Context{c}
		}
		return listRemoteMachines(cmd, contexts, all, running, inventoryProvider, inventoryProject)
	}}
	status.Flags().BoolVar(&all, "all", false, "Also discover machines without saved contexts")
	status.Flags().BoolVar(&running, "running", false, "Show running machines and machines with unknown state")
	status.Flags().StringVar(&inventoryProvider, "provider", "", "Filter by provider")
	status.Flags().StringVar(&inventoryProject, "project", "", "Cloud VPS project to query")
	cmd.AddCommand(status)
	disconnect := &cobra.Command{Use: "disconnect [context]", Short: "Disconnect a background session while retaining remote work", Args: cobra.MaximumNArgs(1), PersistentPreRunE: cobrautil.CallAllPersistentPreRunE, RunE: func(cmd *cobra.Command, args []string) error {
		name := devenv.Context
		if len(args) > 0 {
			name = args[0]
		}
		if err := devremote.Disconnect(cmd.Context(), name); err != nil {
			return err
		}
		cmd.Printf("Disconnected %s. Remote files and containers were retained.\n", name)
		return nil
	}}
	cmd.AddCommand(disconnect)
	syncCmd := &cobra.Command{Use: "sync [context]", Short: "Flush pending local source changes to a connected context", Args: cobra.MaximumNArgs(1), PersistentPreRunE: cobrautil.CallAllPersistentPreRunE, RunE: func(cmd *cobra.Command, args []string) error {
		name := devenv.Context
		if len(args) > 0 {
			name = args[0]
		}
		return devremote.FlushSync(cmd.Context(), name)
	}}
	cmd.AddCommand(syncCmd)
	cmd.AddCommand(newRemoteRemoveCmd(), newRemoteCacheCmd(), remoteMachineAction(false), remoteMachineAction(true))
	return cmd
}
