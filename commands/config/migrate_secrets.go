package config

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewMigrateSecretsCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "migrate [KEY...]",
		Short:   "Move plaintext credentials to the OS keyring",
		Long:    "Move Gerrit, Cloud VPS and Phabricator credentials from config.json to the OS keyring. Each value is verified before its plaintext copy is removed. Supply keys to migrate only those credentials.",
		Example: "migrate --dry-run\nmigrate\nmigrate gerrit.password",
		RunE: func(cmd *cobra.Command, args []string) error {
			disk := config.State().OnDiskKoanf.Copy()
			keys := args
			if len(keys) == 0 {
				keys = disk.Keys()
			} else {
				for _, key := range keys {
					if _, err := config.CredentialForKey(key); err != nil {
						return err
					}
					if disk.String(key) == "" {
						return fmt.Errorf("no plaintext credential to migrate for %s", key)
					}
				}
			}
			sort.Strings(keys)
			count := 0
			for _, key := range keys {
				if !config.SensitiveKey(key) || disk.String(key) == "" {
					continue
				}
				credential, err := config.CredentialForKey(key)
				if err != nil {
					cmd.Printf("Skipped %s: %v\n", key, err)
					continue
				}
				if err := credential.Validate(); err != nil {
					return fmt.Errorf("cannot migrate %s: %w", key, err)
				}
				label := "the OS keyring"
				if dryRun {
					cmd.Printf("Would migrate %s to %s\n", key, label)
				} else {
					if err := credential.MigrateTo(); err != nil {
						return fmt.Errorf("migrate %s: %w", key, err)
					}
					cmd.Printf("Migrated %s to %s\n", key, label)
				}
				count++
			}
			if count == 0 {
				cmd.Println("No supported plaintext credentials to migrate.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show credential keys and destinations without opening providers or moving secrets")
	return cmd
}
