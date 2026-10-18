package config

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewMigrateSecretsCmd() *cobra.Command {
	var dryRun bool
	var provider string
	cmd := &cobra.Command{
		Use:     "migrate [KEY...]",
		Short:   "Move plaintext credentials to their selected provider",
		Long:    "Move Gerrit, Cloud VPS and Phabricator credentials from config.json to their selected provider. Supply credential keys to migrate only those entries. Plaintext entries use the configured default or --provider; secure links retain their provider. Each secret is read back before its plaintext value is removed. This does not import external phab.cfg files or change GitLab's separate credential store.",
		Example: "migrate --dry-run\nmigrate\nmigrate gerrit.password --provider keyring",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "" {
				if err := config.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
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
				target, err := credential.MigrationProvider(provider)
				if err != nil {
					return err
				}
				label := migrationProviderLabel(target)
				if dryRun {
					if target == "" {
						cmd.Printf("Would select storage for %s (no default configured)\n", key)
					} else {
						cmd.Printf("Would migrate %s to %s\n", key, label)
					}
				} else {
					if err := credential.MigrateTo(target); err != nil {
						return fmt.Errorf("migrate %s: %w", key, err)
					}
					actual, err := credential.CredentialProvider("")
					if err != nil {
						return err
					}
					cmd.Printf("Stored %s in %s\n", key, migrationProviderLabel(actual))
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
	cmd.Flags().StringVar(&provider, "provider", "", "Override the credential provider (keyring, keepassxc or plaintext)")
	cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "keepassxc", "plaintext"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func migrationProviderLabel(provider string) string {
	switch provider {
	case "keyring":
		return "the OS keyring"
	case "keepassxc":
		return "KeePassXC"
	case "plaintext":
		return "plaintext configuration"
	default:
		return "the selected provider"
	}
}
