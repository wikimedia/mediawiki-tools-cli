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
		Long:    "Move Gerrit, Cloud VPS and Phabricator credentials from config.json to their selected provider. Supply credential keys to migrate only those entries. Existing links retain their provider; otherwise the configured default is used. Override with --provider. Each secret is read back before its plaintext value is removed. This does not import external phab.cfg files or change GitLab's separate credential store.",
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
				target, err := credential.CredentialProvider(provider)
				if err != nil {
					return err
				}
				label := "the OS keyring"
				if target == "keepassxc" {
					label = "KeePassXC"
				}
				if dryRun {
					cmd.Printf("Would migrate %s to %s\n", key, label)
				} else {
					if err := credential.MigrateTo(target); err != nil {
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
	cmd.Flags().StringVar(&provider, "provider", "", "Override the credential provider (keyring or keepassxc)")
	cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "keepassxc"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
