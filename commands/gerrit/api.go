package gerrit

import (
	_ "embed"
	"fmt"
	"io"
	"log/slog"

	"github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
)

//go:embed api.example
var apiExample string

func NewGerritAPICmd() *cobra.Command {
	var (
		method   string
		user     string
		password string
	)

	cmd := &cobra.Command{
		Use:     "api",
		Short:   "Gerrit's API",
		Example: apiExample,
		Long:    `https://gerrit.wikimedia.org/r/Documentation/rest-api.html`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if user == "" {
				slog.Debug("Using username from config")
				user = gerritUsername
			}
			if password == "" {
				slog.Debug("Using password from config")
				password = gerritPassword
			}

			client, err := newGerritClient(cmd.Context())
			if err != nil {
				return err
			}
			if user != "" && password != "" {
				client.Authentication.SetBasicAuth(user, password)
				slog.Debug("Using username and password")
			}

			resp, err := client.Call(cmd.Context(), method, args[0], nil, nil)
			if err != nil {
				return fmt.Errorf("Gerrit API request: %w", err)
			}

			defer resp.Response.Body.Close()

			b, err := io.ReadAll(resp.Response.Body)
			if err != nil {
				return err
			}
			b = gerrit.RemoveMagicPrefixLine(b)

			// printing the structure
			_, err = cmd.OutOrStdout().Write(b)
			return err
		},
	}
	cmd.Flags().StringVarP(&method, "method", "X", "GET", "The HTTP method for the request")
	cmd.Flags().StringVarP(&user, "auth-user", "", "", "Gerrit HTTP user")
	cmd.Flags().StringVarP(&password, "auth-password", "", "", "Gerrit HTTP password")
	return cmd
}
