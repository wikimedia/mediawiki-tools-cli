package gerrit

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewGerritAuthStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Check the selected Gerrit account's HTTP credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if gerritUsername == "" || gerritPassword == "" {
			return fmt.Errorf("no credentials found; use gerrit auth login")
		}
		client, err := newGerritClient(cmd.Context())
		if err != nil {
			return err
		}
		client.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
		account, response, err := client.Accounts.GetAccount(cmd.Context(), "self")
		if response != nil && response.Response != nil && response.Body != nil {
			response.Body.Close()
		}
		if err != nil {
			return fmt.Errorf("not authenticated: %w", err)
		}
		if account == nil || account.Username == "" {
			return fmt.Errorf("Gerrit returned no authenticated username")
		}
		if !strings.EqualFold(account.Username, gerritUsername) {
			return fmt.Errorf("authenticated as %s, expected %s", account.Username, gerritUsername)
		}
		if name, err := selectedGerritProfile(cmd); err == nil {
			cmd.Println("Profile:", name)
		}
		cmd.Printf("Authenticated as %s on %s\n", account.Username, gerritProfiles.Endpoint)
		return nil
	}}
}
