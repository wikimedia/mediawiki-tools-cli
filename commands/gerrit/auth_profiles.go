package gerrit

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func newGerritAuthProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use: "profiles", Aliases: []string{"list"}, Short: "Show the configured Gerrit account without retrieving credentials", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			username := config.State().EffectiveKoanf.String("gerrit.username")
			if username == "" {
				cmd.Println("No Gerrit account configured. Use mw gerrit auth login.")
				return nil
			}
			cmd.Println("Configured Gerrit account:")
			cmd.Println("  " + username)
			cmd.Println("Gerrit currently supports one saved account; named profile selection is not yet available.")
			return nil
		},
	}
}
