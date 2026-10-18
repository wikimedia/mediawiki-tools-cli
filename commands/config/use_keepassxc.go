package config

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewUseKeePassXCCmd() *cobra.Command {
	var entry string
	cmd := &cobra.Command{
		Use:     "use-keepassxc KEY",
		Short:   "Link a credential to a KeePassXC entry",
		Long:    "Pair first with config secrets keepassxc connect. Select the database and approve access in KeePassXC. Any saved plaintext must match the entry before it is removed. Account metadata must already be configured. See help guides authentication.",
		Example: "use-keepassxc gerrit.password\nuse-keepassxc gerrit.password --entry Wikimedia/Gerrit",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credential, err := config.CredentialForKey(args[0])
			if err != nil {
				return err
			}
			if err := credential.UseKeePassXC(cmd.Context(), entry); err != nil {
				return err
			}
			cmd.Printf("Linked %s to KeePassXC.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "Full entry path (default: wm/KEY, or the previously linked entry)")
	return cmd
}
