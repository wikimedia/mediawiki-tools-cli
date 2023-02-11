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
			client, _ := gerrit.NewClient(instance, nil)
			client.Authentication.SetBasicAuth(config.Username, config.Password)
			_, err := client.Call("GET", "accounts/self/name", nil, nil)

			if err != nil {
				cmd.Println("Not authenticated")
				return
			} else {
				cmd.Println("Authenticated =]")
				return
			}
		},
	}
	return cmd
}
