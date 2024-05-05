package gerrit

import (
	"github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
)

func NewGerritAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status of Wikimedia Gerrit authentication using HTTP credentials",
		Run: func(cmd *cobra.Command, args []string) {
			config := LoadConfig()

			hasCredentials := config.Username != "" && config.Password != ""

			if hasCredentials {
				cmd.Println("Credentials found")
				cmd.Println("Username:", config.Username)
				cmd.Println("Password:", "***...")
			} else {
				cmd.Println("No credentials found")
				return
			}

			instance := "https://gerrit.wikimedia.org/r/"
			client, _ := gerrit.NewClient(cmd.Context(), instance, nil)
			client.Authentication.SetBasicAuth(config.Username, config.Password)
			response, err := client.Call(cmd.Context(), "GET", "accounts/self/name", nil, nil)

			if err != nil {
				if response.StatusCode == 401 {
					cmd.Println("Not authenticated")
				} else {
					cmd.Println(response.StatusCode)
					cmd.PrintErrln(err)
					cmd.Println("Possibly not authenticated?")
				}
				return
			} else {
				cmd.Println("Authenticated =]")
				return
			}
		},
	}
	return cmd
}
