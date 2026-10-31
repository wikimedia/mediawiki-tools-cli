package gerrit

import (
	"github.com/spf13/cobra"
)

func NewGerritAuthCmd(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate mw with Wikimedia Gerrit",
	}
	cmd.AddCommand(NewGerritAuthLoginCmd())
	cmd.AddCommand(gerritProfiles.RemoveCmd(profile))
	cmd.AddCommand(NewGerritAuthStatusCmd())
	cmd.AddCommand(gerritProfiles.ProfilesCmd())
	cmd.AddCommand(gerritProfiles.DefaultCmd(profile))
	return cmd
}
