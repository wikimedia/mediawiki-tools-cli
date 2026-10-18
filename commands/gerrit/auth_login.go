package gerrit

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func NewGerritAuthLoginCmd() *cobra.Command {
	var username, password string
	var passwordStdin, insecureStorage bool
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Login to Wikimedia Gerrit using HTTP credentials",
		Example: "login\nlogin --username myname --password-stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			if passwordStdin {
				var err error
				password, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			if username == "" {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --username")
				}
				if err := survey.AskOne(&survey.Input{Message: "Gerrit username:"}, &username, survey.WithValidator(survey.Required)); err != nil {
					return err
				}
			}
			if password == "" {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --password-stdin or --password")
				}
				cmd.Println("Create an HTTP password at https://gerrit.wikimedia.org/r/settings/#HTTPCredentials")
				if err := survey.AskOne(&survey.Password{Message: "Gerrit HTTP password:"}, &password, survey.WithValidator(survey.Required)); err != nil {
					return err
				}
			}
			client, err := newGerritClient(cmd.Context())
			if err != nil {
				return err
			}
			client.Authentication.SetBasicAuth(username, password)
			response, err := client.Call(cmd.Context(), "GET", "accounts/self/name", nil, nil)
			if response != nil && response.Response != nil && response.Response.Body != nil {
				response.Response.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("Gerrit credentials were not validated or saved: %w", err)
			}
			credential := config.Credential{Key: "gerrit.password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: username}
			if err := credential.SaveWithMetadata(password, insecureStorage, map[string]string{"gerrit.username": username}); err != nil {
				return err
			}
			cmd.Println("Credentials saved")
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Gerrit username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Gerrit HTTP password (prefer a hidden prompt or stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the password from stdin")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Store the password in the private plaintext config using insecure storage")
	cmd.MarkFlagsMutuallyExclusive("password", "password-stdin")
	return cmd
}
