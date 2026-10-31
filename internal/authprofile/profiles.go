// Package authprofile supplies the common CLI contract for saved service accounts.
package authprofile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/knadh/koanf"
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

// Profile describes public account metadata; listing never retrieves credentials.
type Profile struct {
	Name, Username, Endpoint, Details string
	Default                           bool
}

// Backend keeps service-specific storage and realm selection out of the CLI layout.
// Manager implements this for flat service profiles; wiki supplies a realm adapter.
type Backend interface {
	Profiles() ([]Profile, error)
	Selected(string) (string, error)
	SetDefault(string) error
	Remove(string) (bool, error)
}

var _ Backend = Manager{}

type Commands struct {
	Label    string
	Selected *string
	Backend  Backend
}

func (m Manager) Profiles() ([]Profile, error) {
	var result []Profile
	state := config.State().EffectiveKoanf
	for _, name := range m.Names() {
		prefix := m.Prefix(name) + "."
		endpoint := state.String(prefix + "url")
		if endpoint == "" {
			endpoint = m.Endpoint
		}
		result = append(result, Profile{Name: name, Username: state.String(prefix + "username"), Endpoint: endpoint, Default: name == m.Default()})
	}
	return result, nil
}

func (m Manager) SetDefault(name string) error {
	return config.UpdateDiskConfig(func(disk *koanf.Koanf) error {
		if _, err := m.Selected(name); err != nil {
			return err
		}
		return disk.Set(m.Service+".default_profile", name)
	})
}

func (m Manager) Remove(name string) (bool, error) {
	prefix := m.Prefix(name)
	removedDefault := false
	err := config.RemoveCredentialProfile(prefix+"."+m.SecretField, func(disk *koanf.Koanf) error {
		disk.Delete(prefix)
		removedDefault = disk.String(m.Service+".default_profile") == name
		if removedDefault {
			disk.Delete(m.Service + ".default_profile")
		}
		return nil
	})
	return removedDefault, err
}

func (c Commands) ProfilesCmd() *cobra.Command {
	return &cobra.Command{Use: "profiles", Aliases: []string{"list"}, Short: "List saved " + c.Label + " profiles without retrieving credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		profiles, err := c.Backend.Profiles()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			cmd.Println("No saved " + c.Label + " profiles. Use auth login.")
			return nil
		}
		cmd.Println("Saved " + c.Label + " profiles:")
		for _, profile := range profiles {
			marker := ""
			if profile.Default {
				marker = " (default)"
			}
			details := ""
			if profile.Details != "" {
				details = " — " + profile.Details
			}
			cmd.Printf("  %s%s — %s — %s%s\n", profile.Name, marker, profile.Username, profile.Endpoint, details)
		}
		return nil
	}}
}

func (c Commands) DefaultCmd() *cobra.Command {
	return &cobra.Command{Use: "default", Short: "Select the default saved profile with --profile", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(*c.Selected) == "" {
			return fmt.Errorf("provide --profile")
		}
		name, err := c.Backend.Selected(*c.Selected)
		if err != nil {
			return err
		}
		if err := c.Backend.SetDefault(name); err != nil {
			return err
		}
		cmd.Printf("Default profile: %s\n", name)
		return nil
	}}
}

func (c Commands) RemoveCmd() *cobra.Command {
	return &cobra.Command{Use: "remove", Short: "Remove a saved " + c.Label + " profile locally", Long: "Remove the selected profile and its local credential or reference. Server-side credentials and external KeePassXC entries are retained.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		name, err := c.Backend.Selected(*c.Selected)
		if err != nil {
			return err
		}
		removedDefault, err := c.Backend.Remove(name)
		if err != nil {
			return err
		}
		cmd.Printf("Removed local profile %q. Server-side credentials are not revoked; external KeePassXC entries are retained. Review them with config secrets detached.\n", name)
		if removedDefault {
			cmd.Println("The default profile was removed; use auth default --profile NAME to choose another.")
		}
		return nil
	}}
}

// AddStorageFlags keeps credential provider selection identical across services.
func AddStorageFlags(cmd *cobra.Command, provider *string, insecure *bool) {
	cmd.Flags().StringVar(provider, "provider", "", "Override the credential provider (keyring, windows-credential-manager, keepassxc or plaintext)")
	cmd.Flags().BoolVar(insecure, "insecure-storage", false, "Store credentials in the private plaintext configuration")
	cmd.MarkFlagsMutuallyExclusive("provider", "insecure-storage")
	_ = cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "windows-credential-manager", "keepassxc", "plaintext"}, cobra.ShellCompDirectiveNoFileComp
	})
}
