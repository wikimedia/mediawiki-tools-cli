package phabricator

import (
	"fmt"

	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func newPhabricatorAuthRemoveCmd(profile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove a saved Phabricator profile locally",
		Long:  "Remove the selected profile and its local credential or reference. This does not revoke the token in Phabricator or delete an external KeePassXC entry.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name := resolveAuthSite(*profile)
			if !validProfileName(name) {
				return fmt.Errorf("invalid profile name")
			}
			prefix := "phabricator.profiles." + name
			key := prefix + ".key"
			if !mwconfig.State().OnDiskKoanf.Exists(prefix) && !mwconfig.HasCredential(key) {
				return fmt.Errorf("no saved Phabricator profile %q", name)
			}
			if mwconfig.HasCredential(key) {
				credential, err := mwconfig.CredentialForKey(key)
				if err != nil {
					return err
				}
				if err := credential.Delete(); err != nil {
					return err
				}
			}
			disk := mwconfig.State().OnDiskKoanf.Copy()
			disk.Delete(prefix)
			removedDefault := disk.String("phabricator.default_profile") == name
			if removedDefault {
				disk.Delete("phabricator.default_profile")
			}
			if err := mwconfig.PutDiskConfig(disk); err != nil {
				return err
			}
			cmd.Printf("Removed local profile %q. The token remains valid in Phabricator; external KeePassXC entries are retained.\n", name)
			if removedDefault {
				cmd.Println("The default profile was removed; select another with --profile or set phabricator.default_profile.")
			}
			return nil
		},
	}
}
