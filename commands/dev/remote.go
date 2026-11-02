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
	var host, project, instance, user, jump, directory, sourceName, gitRemote string
	var forwarded []string
	open := &cobra.Command{
		Use: "open [context]", Short: "Prepare or reconnect a background remote development session",
		Long: `Keep code and containers on a Linux VM, with browser access maintained by a background SSH session.
New contexts receive a two-word name and use ~/wm/CONTEXT/mediawiki by default. Reopen a saved context without supplying its SSH target again.
Use --directory for continuous local-to-remote source synchronization. Git metadata, dotenv files and dependency directories are excluded; overwritten remote files are retained under .wm/sync-backup. Local-folder mode defaults to no recipe and opens the local editor.
Use --editor web for a browser editor on the VM, code for desktop VS Code, or none. Editor access is authenticated and forwarded through SSH.
Stable releases download the exact release’s Linux build for the VM. Development and nightly builds copy this executable and require matching Linux OS/architecture. --install-deps installs Git and Docker on supported Debian/Ubuntu VMs using passwordless sudo.
Phabricator commands can execute on this machine through the active session bridge (--forward phab, the default). Credential management and shell commands are not delegated; file arguments are transferred as stdin. --forward none disables delegation.
--git-remote selects an authenticated Git destination on this machine. Fetch/push transfer objects through a local mirror; no service credentials or SSH agents are copied to the VM.`,
		Example: cobrautil.NormalizeExample(`open --ssh awm-1.boxd --recipe mediawiki
open --project mwcli --instance ea86a663-eee4-4ed7-9962-d7f339800b5c --editor web
open calm-otter
open --ssh my-vm --directory . --recipe none --editor none`),
		Args: cobra.MaximumNArgs(1), PersistentPreRun: cobrautil.CallAllPersistentPreRun,
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
			target := host != "" || instance != ""
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
			c, err := devremote.LoadContext(name)
			fresh := os.IsNotExist(err)
			if err != nil && !fresh {
				return err
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
			if target {
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
				return fmt.Errorf("a new context needs --ssh or --project with --instance")
			}
			previousEditor := c.Editor
			previousGit := c.GitRemote
			if !fresh && cmd.Flags().Changed("recipe") && c.Recipe != opts.Recipe {
				return fmt.Errorf("use another context for another recipe")
			}
			if fresh || cmd.Flags().Changed("recipe") {
				c.Recipe = opts.Recipe
			}
			if fresh || cmd.Flags().Changed("editor") {
				c.Editor = opts.Editor
			}
			c.NoBrowser = opts.NoBrowser
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
			if fresh || cmd.Flags().Changed("forward") {
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
			c.InstallDeps = opts.InstallDeps
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
			return devremote.StartSession(cmd.Context(), c, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	open.Flags().StringVar(&host, "ssh", "", "SSH host or configured alias")
	open.Flags().StringVar(&project, "project", "", "Cloud VPS project (with --instance)")
	open.Flags().StringVar(&instance, "instance", "", "Cloud VPS instance ID")
	open.Flags().StringVar(&user, "user", "", "Cloud VPS SSH username")
	open.Flags().StringVar(&jump, "jump-host", "", "Cloud VPS SSH bastion override")
	open.Flags().StringVar(&opts.Recipe, "recipe", "mediawiki", "Built-in recipe, or none for an existing/arbitrary project")
	open.Flags().IntVar(&opts.Port, "port", 0, "Environment port (first setup only; automatically selected by default)")
	open.Flags().StringVar(&opts.Editor, "editor", "code", "Editor: code, web or none")
	open.Flags().BoolVar(&opts.NoBrowser, "no-browser", false, "Print URLs without opening the browser")
	open.Flags().BoolVar(&opts.InstallDeps, "install-deps", false, "Install remote dependencies using passwordless sudo")
	open.Flags().StringVar(&directory, "directory", "", "Continuously synchronize this local project directory to the VM")
	open.Flags().StringVar(&sourceName, "source-name", "mediawiki", "Name of the codebase directory within the context")
	open.Flags().StringSliceVar(&forwarded, "forward", []string{"phab"}, "Locally executed services: phab or none")
	open.Flags().StringVar(&gitRemote, "git-remote", "", "Git destination authenticated locally (enables the Git bridge)")
	cmd.AddCommand(open)
	status := &cobra.Command{Use: "status [context]", Aliases: []string{"list", "contexts"}, Short: "List remote contexts and their connection status", Args: cobra.MaximumNArgs(1), PersistentPreRun: cobrautil.CallAllPersistentPreRun, RunE: func(cmd *cobra.Command, args []string) error {
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
		for _, c := range contexts {
			line, err := devremote.SessionStatus(cmd.Context(), c.Context)
			if err != nil {
				return err
			}
			cmd.Println(line)
		}
		return nil
	}}
	cmd.AddCommand(status)
	disconnect := &cobra.Command{Use: "disconnect [context]", Short: "Disconnect a background session while retaining remote work", Args: cobra.MaximumNArgs(1), PersistentPreRun: cobrautil.CallAllPersistentPreRun, RunE: func(cmd *cobra.Command, args []string) error {
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
	syncCmd := &cobra.Command{Use: "sync [context]", Short: "Flush pending local source changes to a connected context", Args: cobra.MaximumNArgs(1), PersistentPreRun: cobrautil.CallAllPersistentPreRun, RunE: func(cmd *cobra.Command, args []string) error {
		name := devenv.Context
		if len(args) > 0 {
			name = args[0]
		}
		return devremote.FlushSync(cmd.Context(), name)
	}}
	cmd.AddCommand(syncCmd)
	cmd.AddCommand(newRemoteRemoveCmd(), newRemoteCacheCmd())
	return cmd
}
