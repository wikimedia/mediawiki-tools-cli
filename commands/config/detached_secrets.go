package config

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewDetachedSecretsCmd() *cobra.Command {
	var details, asJSON bool
	var provider string
	cmd := &cobra.Command{
		Use:   "detached",
		Short: "List retained credentials not linked by this configuration",
		Long:  "List metadata for entries retained by transfers, relinking, or profile removal. --provider filters retained entries and requests provider discovery where supported. Windows discovers CLI-owned targets; KeePassXC uses permission-gated metadata enumeration and shows only CLI-tagged URLs. Secret values are never displayed or stored in the retention record.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Keep the table, details, and suggested commands on the same stream.
			cmd.SetOut(cmd.OutOrStdout())
			entries, err := config.DetachedSecrets(provider)
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
			}
			cmd.Println("Not referenced by this configuration; other configurations or applications may still use these entries.")
			if len(entries) == 0 {
				cmd.Println("No detached entries found.")
			} else {
				table := output.NewTable([]interface{}{"ID", "KEY", "PROVIDER", "REASON"}, nil)
				for _, entry := range entries {
					key := entry.Key
					if key == "" {
						key = "(unmapped)"
					}
					table.AddRow(secretColumn(entry.ID[:12]), secretColumn(key), secretColumn(entry.Provider), secretColumn(entry.Reason))
				}
				table.Print(cmd.OutOrStdout())
				for _, entry := range entries {
					if details {
						cmd.Printf("\n%s\n  Account: %s\n", entry.ID, secretColumn(entry.Account))
						if entry.Target != "" {
							cmd.Printf("  Target: %s\n", secretColumn(entry.Target))
						}
						if entry.EntryID != "" {
							cmd.Printf("  Database: %s\n  Entry: %s\n", secretColumn(entry.DatabaseID), secretColumn(entry.EntryID))
						}
						if entry.RetainedAt != "" {
							cmd.Printf("  Retained: %s\n", secretColumn(entry.RetainedAt))
						}
					}
					scan := " --provider " + shellWord(entry.Provider)
					cmd.Printf("\n  # Delete detached %s entry\n  $ %s config secrets cleanup %s%s --yes\n", secretColumn(entry.Provider), shellWord(cmd.Root().Name()), entry.ID[:12], scan)
				}
			}
			if provider == "keepassxc" {
				cmd.Println("\nKeePassXC discovery includes CLI-tagged URLs; older untagged entries still need manual review.")
			} else if provider == "" {
				cmd.Println("\nUse --provider to discover provider entries as well as tracked copies. Native Linux keyring discovery covers tracked entries only.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Filter and discover entries in keyring, windows-credential-manager or keepassxc")
	cmd.RegisterFlagCompletionFunc("provider", detachedProviderCompletion)
	cmd.Flags().BoolVar(&details, "details", false, "Show store target names, database IDs, and entry UUIDs")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output public detached-entry metadata as JSON")
	return cmd
}

func NewCleanupSecretCmd() *cobra.Command {
	var yes bool
	var provider string
	cmd := &cobra.Command{
		Use:   "cleanup ID",
		Short: "Delete one explicitly selected detached credential",
		Long:  "Delete one entry selected from secrets detached. Rechecks all current credential references under a mutation lock and refuses linked or ambiguous entries. --yes explicitly approves deletion; KeePassXC also requests GUI approval. This removes the stored copy, not the server-side credential.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("review config secrets detached --details, then specify its ID and --yes to approve deletion")
			}
			if err := config.CleanupDetached(cmd.Context(), args[0], provider); err != nil {
				return err
			}
			cmd.Println("Detached credential removed. Server-side credentials were not revoked.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Explicitly approve deletion of the selected detached entry")
	cmd.Flags().StringVar(&provider, "provider", "", "Select the provider and rediscover an untracked entry if needed")
	cmd.RegisterFlagCompletionFunc("provider", detachedProviderCompletion)
	return cmd
}

func detachedProviderCompletion(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return []string{"keyring", "windows-credential-manager", "keepassxc"}, cobra.ShellCompDirectiveNoFileComp
}
