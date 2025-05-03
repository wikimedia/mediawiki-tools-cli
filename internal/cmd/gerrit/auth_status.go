package gerrit

import (
	"fmt"

	"github.com/andygrunwald/go-gerrit"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
)

func NewGerritAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status of Wikimedia Gerrit authentication using HTTP credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			hasCredentials := gerritUsername != "" && gerritPassword != ""

			if hasCredentials {
				cmd.Println("Username:", gerritUsername)
				cmd.Println("Password:", "*************")
			} else {
				return fmt.Errorf("no credentials found")
			}

			instance := "https://gerrit.wikimedia.org/r/"
			client, _ := gerrit.NewClient(cmd.Context(), instance, nil)
			client.Authentication.SetBasicAuth(gerritUsername, gerritPassword)
			response, err := client.Call(cmd.Context(), "GET", "accounts/self/name", nil, nil)
			if err != nil {
				logrus.Debugf("Status Code: %v", response.StatusCode)
				logrus.Debugf("Error: %v", err)
				if response.StatusCode == 401 {
					return fmt.Errorf("401: not authenticated")
				} else {
					return fmt.Errorf("unknown error")
				}
			}
			cmd.Println(cmdgloss.SuccessHeading("Authenticated"))
			return nil
		},
	}
	return cmd
}
