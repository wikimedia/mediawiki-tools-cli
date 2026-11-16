package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewProfilesCmd() *cobra.Command {
	var asJSON, details bool
	var service string
	run := func(cmd *cobra.Command, _ []string) error {
		selected := strings.TrimSpace(service)
		if selected == "phab" {
			selected = "phabricator"
		}
		if selected != "" && selected != "wiki" && selected != "gerrit" && selected != "phabricator" && selected != "cloudvps" && selected != "gitlab" && selected != "enterprise" {
			return fmt.Errorf("--service must be wiki, gerrit, phabricator, gitlab, cloudvps or enterprise")
		}
		profiles := config.ProfileInventory()
		if selected != "" {
			filtered := make([]config.ProfileInfo, 0)
			for _, profile := range profiles {
				if profile.Service == selected {
					filtered = append(filtered, profile)
				}
			}
			profiles = filtered
		}
		if asJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(profiles)
		}
		if len(profiles) == 0 {
			cmd.Println("No saved profiles found.")
			return nil
		}
		table := output.NewTable([]interface{}{"SERVICE", "PROFILE", "ACCOUNT", "PROVIDER", "DEFAULT"}, nil)
		for _, profile := range profiles {
			marker := "-"
			if profile.Default {
				marker = "yes"
			} else if profile.RealmDefault {
				marker = "realm"
			}
			table.AddRow(secretColumn(profile.Service), secretColumn(profile.Name), secretColumn(profile.Account), secretColumn(profile.Provider), marker)
		}
		table.Print(cmd.OutOrStdout())
		if details {
			for _, profile := range profiles {
				cmd.Printf("\n%s / %s\n", secretColumn(profile.Service), secretColumn(profile.Name))
				cmd.Printf("  Method: %s\n  Secret: %s\n  Status: %s\n", secretColumn(profile.Method), secretColumn(profile.SecretKey), secretColumn(profile.Status))
				if profile.Endpoint != "" {
					cmd.Println("  Endpoint:", secretColumn(profile.Endpoint))
				}
				if profile.SiteScope != "" {
					cmd.Println("  Scope:", secretColumn(profile.SiteScope))
				}
				if len(profile.Sites) > 0 {
					cmd.Println("  Sites:", secretColumn(strings.Join(profile.Sites, ", ")))
				}
			}
		}
		cmd.Println("\nProfiles and credential references only; secrets were not retrieved or verified.")
		if !details {
			cmd.Println("Use --details for methods, site scopes and secret keys.")
		}
		return nil
	}
	cmd := &cobra.Command{Use: "profiles", Short: "List saved profiles across all services without retrieving credentials", Args: cobra.NoArgs, RunE: run}
	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "Output profile metadata as JSON")
	cmd.PersistentFlags().BoolVar(&details, "details", false, "Show authentication methods, site scopes and secret keys")
	cmd.PersistentFlags().StringVar(&service, "service", "", "Filter by service: wiki, gerrit, phabricator, gitlab, cloudvps or enterprise")
	_ = cmd.RegisterFlagCompletionFunc("service", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"wiki", "gerrit", "phabricator", "gitlab", "cloudvps", "enterprise"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.AddCommand(&cobra.Command{Use: "list", Aliases: []string{"ls"}, Short: cmd.Short, Args: cobra.NoArgs, RunE: run})
	return cmd
}
