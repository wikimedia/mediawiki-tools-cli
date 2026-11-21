package gerrit

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewGerritAuthStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Check the selected Gerrit account's HTTP credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		username, err := checkGerritHTTP(cmd.Context(), gerritUsername, gerritPassword)
		if err != nil {
			return err
		}
		if name, err := selectedGerritProfile(cmd); err == nil {
			cmd.Println("Profile:", name)
		}
		cmd.Printf("Authenticated as %s on %s\n", username, gerritProfiles.Endpoint)
		return nil
	}}
}

func checkGerritHTTP(ctx context.Context, username, password string) (string, error) {
	if username == "" || password == "" {
		return "", fmt.Errorf("no credentials found; use mw gerrit auth login")
	}
	ctx, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
	defer cancel()
	client, err := newGerritClient(ctx)
	if err != nil {
		return "", err
	}
	client.Authentication.SetBasicAuth(username, password)
	account, response, err := client.Accounts.GetAccount(ctx, "self")
	if response != nil && response.Response != nil && response.Body != nil {
		response.Body.Close()
	}
	if err != nil {
		return "", fmt.Errorf("not authenticated: %w", err)
	}
	if account == nil || account.Username == "" {
		return "", fmt.Errorf("Gerrit returned no authenticated username")
	}
	if !strings.EqualFold(account.Username, username) {
		return "", fmt.Errorf("authenticated as %s, expected %s", account.Username, username)
	}
	return account.Username, nil
}
