// Package authprofile supplies the common CLI contract for saved service accounts.
package authprofile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

type Manager struct {
	Service     string
	Label       string
	SecretField string
	Endpoint    string
}

func ValidName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func (m Manager) Prefix(name string) string { return m.Service + ".profiles." + name }

func (m Manager) Default() string {
	return strings.TrimSpace(config.State().EffectiveKoanf.String(m.Service + ".default_profile"))
}

func (m Manager) Names() []string {
	names := map[string]bool{}
	state := config.State()
	for _, conf := range [][]string{state.OnDiskKoanf.MapKeys(m.Service + ".profiles"), state.EffectiveKoanf.MapKeys(m.Service + ".profiles")} {
		for _, name := range conf {
			names[name] = true
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func (m Manager) Selected(name string) (string, error) {
	if name == "" {
		name = m.Default()
	}
	if !ValidName(name) {
		return "", fmt.Errorf("select a saved %s account with --profile or use auth login", m.Label)
	}
	for _, candidate := range m.Names() {
		if candidate == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("no saved %s profile %q", m.Label, name)
}

func (m Manager) Complete(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	var result []string
	for _, name := range m.Names() {
		if strings.HasPrefix(name, prefix) {
			result = append(result, name)
		}
	}
	return result, cobra.ShellCompDirectiveNoFileComp
}

func (m Manager) AddFlag(parent *cobra.Command, selected *string) {
	parent.PersistentFlags().StringVar(selected, "profile", "", "Saved account profile (login defaults to the verified username)")
	_ = parent.RegisterFlagCompletionFunc("profile", m.Complete)
}

// ChooseDefault is shared by login commands; service-specific identity checks
// finish before confirmation, and credentials/default metadata commit together.
func ChooseDefault(cmd *cobra.Command, name, current string, canAsk func() bool, confirm func(string, string) (bool, error)) (bool, error) {
	if cmd.Flags().Changed("default") {
		return cmd.Flags().GetBool("default")
	}
	if current == "" || current == name {
		return true, nil
	}
	if canAsk() {
		return confirm(name, current)
	}
	return false, nil
}

func ConfirmDefault(name, current string) (bool, error) {
	var yes bool
	err := survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("Make profile %q the default instead of %q?", name, current), Default: false}, &yes)
	return yes, err
}

func ChooseLoginDefault(cmd *cobra.Command, name, current string) (bool, error) {
	return ChooseDefault(cmd, name, current, cli.CanAskForInput, ConfirmDefault)
}

func AddDefaultFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("default", false, "Make this profile the default (otherwise ask before switching an existing default)")
}

func (m Manager) ProfilesCmd() *cobra.Command {
	return &cobra.Command{Use: "profiles", Aliases: []string{"list"}, Short: "List saved " + m.Label + " profiles without retrieving credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		names := m.Names()
		if len(names) == 0 {
			cmd.Println("No saved " + m.Label + " profiles. Use auth login.")
			return nil
		}
		cmd.Println("Saved " + m.Label + " profiles:")
		state := config.State().EffectiveKoanf
		for _, name := range names {
			prefix := m.Prefix(name) + "."
			endpoint := state.String(prefix + "url")
			if endpoint == "" {
				endpoint = m.Endpoint
			}
			marker := ""
			if name == m.Default() {
				marker = " (default)"
			}
			cmd.Printf("  %s%s — %s — %s\n", name, marker, state.String(prefix+"username"), endpoint)
		}
		return nil
	}}
}

func (m Manager) DefaultCmd(selected *string) *cobra.Command {
	return &cobra.Command{Use: "default", Short: "Select the default saved account with --profile", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(*selected) == "" {
			return fmt.Errorf("provide --profile")
		}
		name, err := m.Selected(*selected)
		if err != nil {
			return err
		}
		if err := config.PutKeyValueOnDisk(m.Service+".default_profile", name); err != nil {
			return err
		}
		cmd.Printf("Default profile: %s\n", name)
		return nil
	}}
}

func (m Manager) RemoveCmd(selected *string) *cobra.Command {
	return &cobra.Command{Use: "remove", Short: "Remove a saved " + m.Label + " profile locally", Long: "Remove the selected profile and its local credential or reference. Server-side credentials and external KeePassXC entries are retained.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		name, err := m.Selected(*selected)
		if err != nil {
			return err
		}
		prefix := m.Prefix(name)
		key := prefix + "." + m.SecretField
		if config.HasCredential(key) {
			credential, err := config.CredentialForKey(key)
			if err != nil {
				return err
			}
			if err := credential.Delete(); err != nil {
				return err
			}
		}
		disk := config.State().OnDiskKoanf.Copy()
		disk.Delete(prefix)
		removedDefault := disk.String(m.Service+".default_profile") == name
		if removedDefault {
			disk.Delete(m.Service + ".default_profile")
		}
		if err := config.PutDiskConfig(disk); err != nil {
			return err
		}
		cmd.Printf("Removed local profile %q. Server-side credentials are not revoked; external KeePassXC entries are retained.\n", name)
		if removedDefault {
			cmd.Printf("The default profile was removed; use auth default --profile NAME to choose another.\n")
		}
		return nil
	}}
}
