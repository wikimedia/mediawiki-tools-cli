package gerrit

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
)

func NewGerritAuthCmd(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate mw with Wikimedia Gerrit",
	}
	profiles := authprofile.Commands{Label: "Gerrit", Selected: profile, Backend: gerritProfiles}
	cmd.AddCommand(NewGerritAuthLoginCmd())
	cmd.AddCommand(profiles.RemoveCmd())
	cmd.AddCommand(NewGerritAuthStatusCmd())
	cmd.AddCommand(profiles.ProfilesCmd())
	cmd.AddCommand(profiles.DefaultCmd())
	return cmd
}
