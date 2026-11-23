package dev

import (
	"fmt"
	"os"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/cloudvps"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devremote"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

var remoteProviders = []string{"lima", "codespaces", "boxd", "cloudvps"}

func canonicalRemoteProvider(provider string) string {
	if provider == "github" {
		return "codespaces"
	}
	return provider
}

type providerOptions struct {
	kind, target, repo, machine, image, flavor, network, key, userData string
	idle                                                               time.Duration
}

func (p *providerOptions) addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&p.kind, "provider", "", "Machine provider: lima, codespaces (github), boxd or cloudvps")
	f.StringVar(&p.target, "target", "", "Existing provider machine name or ID (Cloud VPS uses the instance ID)")
	f.StringVar(&p.repo, "repo", "", "GitHub repository (OWNER/REPO, defaults to github/codespaces-blank)")
	f.StringVar(&p.machine, "machine", "", "Override the Codespaces machine type")
	f.DurationVar(&p.idle, "idle-timeout", 0, "Provider inactivity limit for a new Codespace or Boxd machine")
	f.StringVar(&p.image, "image", "", "Override the default Cloud VPS image ID")
	f.StringVar(&p.flavor, "flavor", "", "Override the default Cloud VPS flavor ID")
	f.StringVar(&p.network, "network", "", "Override the default Cloud VPS network ID")
	f.StringVar(&p.key, "key-name", "", "Cloud VPS SSH key pair name")
	f.StringVar(&p.userData, "user-data", "", "Cloud VPS first-boot user data file")
}

func (p providerOptions) validate(cmd *cobra.Command, creating bool, project, host, instance, lima, user, jump string) error {
	if p.kind != "" && !slices.Contains(remoteProviders, p.kind) {
		return fmt.Errorf("unknown machine provider %q", p.kind)
	}
	if creating && (p.kind == "" || p.target != "") {
		return fmt.Errorf("create requires --provider and takes no --target")
	}
	if !creating && ((p.kind == "") != (p.target == "")) {
		return fmt.Errorf("opening an existing provider machine requires both --provider and --target")
	}
	if p.kind != "" && (host != "" || instance != "" || lima != "") {
		return fmt.Errorf("use --provider with --target, or use the SSH, Lima or Cloud VPS target flags")
	}
	if p.kind != "" && p.kind != "cloudvps" && (project != "" || user != "" || jump != "") {
		return fmt.Errorf("--project, --user and --jump-host are Cloud VPS options")
	}
	if p.kind == "cloudvps" && project == "" {
		return fmt.Errorf("the Cloud VPS provider requires --project")
	}
	if (p.repo != "" || p.machine != "") && (!creating || p.kind != "codespaces") {
		return fmt.Errorf("--repo and --machine are for creating a Codespace")
	}
	if (p.image != "" || p.flavor != "" || p.network != "" || p.key != "" || p.userData != "") && (!creating || p.kind != "cloudvps") {
		return fmt.Errorf("--image, --flavor, --network, --key-name and --user-data are for creating a Cloud VPS instance")
	}
	if cmd.Flags().Changed("idle-timeout") && (!creating || (p.kind != "boxd" && p.kind != "codespaces") || p.idle <= 0) {
		return fmt.Errorf("--idle-timeout requires a positive duration when creating a Codespace or Boxd machine")
	}
	return nil
}

func (p providerOptions) provision(cmd *cobra.Command, name, project, user, jump, recipe string) (devremote.ProviderTarget, error) {
	if p.kind == "cloudvps" {
		var data []byte
		if p.userData != "" {
			var err error
			data, err = os.ReadFile(p.userData)
			if err != nil {
				return devremote.ProviderTarget{}, fmt.Errorf("read --user-data: %w", err)
			}
		}
		id, err := cloudvps.CreateRemoteInstance(cmd, project, name, p.image, p.flavor, p.network, p.key, data)
		return devremote.ProviderTarget{Kind: p.kind, ID: id, Project: project, User: user, Jump: jump, Owned: true, WaitCloudInit: p.userData != "" || p.image == ""}, err
	}
	return devremote.CreateProviderMachine(cmd.Context(), p.kind, name, p.repo, p.machine, recipe, p.idle, cmd.ErrOrStderr())
}

func providerSSH(cmd *cobra.Command, c devremote.Context) ([]string, error) {
	p := *c.Provider
	if p.Kind != "cloudvps" {
		return devremote.ProviderSSH(cmd.Context(), p, c.Context, cmd.OutOrStdout(), cmd.ErrOrStderr())
	}
	if err := cloudvps.WakeRemoteInstance(cmd, p.Project, p.ID, p.Owned); err != nil {
		return nil, err
	}
	args, err := cloudvps.RemoteSSHArgs(cmd, p.Project, p.ID, p.User, p.Jump)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(args, func(arg string) bool { return arg == "-T" }), nil
}

func listRemoteMachines(cmd *cobra.Command, contexts []devremote.Context, all, running bool, provider, project string) error {
	provider = canonicalRemoteProvider(provider)
	if provider != "" && !slices.Contains(remoteProviders, provider) {
		return fmt.Errorf("unknown machine provider %q", provider)
	}
	type inventoryKey struct{ kind, project string }
	inventory := map[inventoryKey][]devremote.Machine{}
	seen := map[inventoryKey]map[string]bool{}
	failed := false

	load := func(key inventoryKey) []devremote.Machine {
		if machines, ok := inventory[key]; ok {
			return machines
		}
		seen[key] = map[string]bool{}
		var machines []devremote.Machine
		if key.kind == "cloudvps" {
			servers, err := cloudvps.RemoteInstances(cmd, key.project)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not check Cloud VPS project %s: %v\n", key.project, err)
				failed = true
			}
			for _, s := range servers {
				machines = append(machines, devremote.Machine{ID: s.ID, Name: s.Name, State: devremote.MachineState(s.Status)})
			}
		} else {
			var err error
			machines, err = devremote.ProviderMachines(cmd.Context(), devremote.ProviderTarget{Kind: key.kind}, cmd.ErrOrStderr())
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not check %s: %v\n", key.kind, err)
				failed = true
			}
		}
		inventory[key] = machines
		return machines
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CONTEXT\tPROVIDER\tMACHINE\tSTATE\tSESSION\tLAST USED")
	printMachine := func(name, kind string, machine devremote.Machine, session string) {
		if running && machine.State != "running" && machine.State != "unknown" {
			return
		}
		last := "-"
		if !machine.LastUsed.IsZero() {
			last = machine.LastUsed.Local().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, kind, machine.Name, machine.State, session, last)
	}
	for _, c := range contexts {
		p := c.Provider
		if p == nil && c.Lima != "" {
			p = &devremote.ProviderTarget{Kind: "lima", ID: c.Lima}
		}
		if p == nil {
			if provider == "" {
				printMachine(c.Context, "ssh", devremote.Machine{Name: c.SSHArgs[len(c.SSHArgs)-1], State: "unknown"}, devremote.ConnectionState(cmd.Context(), c.Context))
			}
			continue
		}
		if (provider != "" && p.Kind != provider) || (project != "" && p.Kind == "cloudvps" && p.Project != project) {
			continue
		}
		key := inventoryKey{p.Kind, p.Project}
		machines := load(key)
		machine := devremote.Machine{ID: p.ID, Name: p.ID, State: "unknown"}
		for _, m := range machines {
			if m.ID == p.ID && (p.Region == "" || m.Region == p.Region) {
				machine = m
				break
			}
		}
		seen[key][machine.ID+"/"+machine.Region] = true
		printMachine(c.Context, p.Kind, machine, devremote.ConnectionState(cmd.Context(), c.Context))
	}
	if all {
		for _, kind := range remoteProviders {
			if provider != "" && kind != provider {
				continue
			}
			if kind == "cloudvps" && project == "" {
				if provider != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "Specify --project to discover additional Cloud VPS machines.")
					failed = true
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "Cloud VPS discovery covers saved projects; specify --project to scan another.")
				}
				continue
			}
			load(inventoryKey{kind, projectForProvider(kind, project)})
		}
		for key, machines := range inventory {
			for _, m := range machines {
				if !seen[key][m.ID+"/"+m.Region] {
					printMachine("-", key.kind, m, "-")
				}
			}
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("some machine states could not be checked")
	}
	return nil
}

func projectForProvider(kind, project string) string {
	if kind == "cloudvps" {
		return project
	}
	return ""
}

func remoteMachineAction(destroy bool) *cobra.Command {
	action, short := "stop", "Stop a machine created by mw and retain its disk"
	if destroy {
		action, short = "destroy", "Delete a machine created by mw and its remote data"
	}
	var yes bool
	cmd := &cobra.Command{
		Use:               action + " CONTEXT",
		Short:             short,
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: cobrautil.CallAllPersistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			unlock, err := devremote.LockProvider(cmd.Context(), name)
			if err != nil {
				return err
			}
			defer unlock()
			c, err := devremote.LoadContext(name)
			if err != nil {
				return err
			}
			if c.Provider == nil || !c.Provider.Owned {
				return fmt.Errorf("context %s uses an existing machine; only machines created by mw can be stopped or destroyed here", name)
			}
			contexts, err := devremote.Contexts()
			if err != nil {
				return err
			}
			for _, other := range contexts {
				if other.Context != name && other.Provider != nil && c.Provider.Kind == other.Provider.Kind && c.Provider.ID == other.Provider.ID && c.Provider.Project == other.Provider.Project && (c.Provider.Region == "" || other.Provider.Region == "" || c.Provider.Region == other.Provider.Region) {
					return fmt.Errorf("context %s also uses this machine", other.Context)
				}
			}
			if destroy {
				if err := confirmRemoteCleanup(yes, fmt.Sprintf("Permanently delete %s machine %s and its data?", c.Provider.Kind, c.Provider.ID)); err != nil {
					return err
				}
			}
			if devremote.ConnectionState(cmd.Context(), name) != "disconnected" {
				if err := devremote.Disconnect(cmd.Context(), name); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			if c.Provider.Kind == "cloudvps" {
				err = cloudvps.ChangeRemoteInstance(cmd, c.Provider.Project, c.Provider.ID, destroy)
			} else {
				err = devremote.ChangeProviderMachine(cmd.Context(), *c.Provider, destroy, cmd.ErrOrStderr())
			}
			if err != nil {
				return err
			}
			if destroy {
				return devremote.RemoveContext(cmd.Context(), name, false, cmd.OutOrStdout(), cmd.ErrOrStderr())
			}
			cmd.Printf("Stop requested for %s machine %s.\n", c.Provider.Kind, c.Provider.ID)
			return nil
		},
	}
	if destroy {
		cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm permanent deletion")
	}
	return cmd
}
