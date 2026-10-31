package gerrit

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func NewGerritAuthLoginCmd() *cobra.Command {
	var username, password string
	var passwordStdin, insecureStorage bool
	var provider string
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Login to Wikimedia Gerrit using HTTP credentials",
		Example: "login\nlogin --username myname --password-stdin",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "" {
				if err := config.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
			if passwordStdin {
				var err error
				password, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			username = strings.TrimSpace(username)
			if name, _ := cmd.Flags().GetString("profile"); name != "" {
				if err := requireGerritProfileName(name); err != nil {
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
			cmd.Println("Checking the authenticated Gerrit identity...")
			account, response, err := client.Accounts.GetAccount(cmd.Context(), "self")
			if response != nil && response.Response != nil && response.Body != nil {
				response.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("Gerrit credentials were not validated or saved: %w", err)
			}
			if account == nil || strings.TrimSpace(account.Username) == "" {
				return fmt.Errorf("Gerrit did not return an authenticated username; credentials were not saved")
			}
			if !strings.EqualFold(username, account.Username) {
				return fmt.Errorf("credentials belong to %s, expected %s; credentials were not saved", account.Username, username)
			}
			username = account.Username
			name, _ := cmd.Flags().GetString("profile")
			if name == "" {
				name = strings.ToLower(username)
			}
			if err := requireGerritProfileName(name); err != nil {
				return err
			}
			current := gerritProfiles.Default()
			changeDefault, err := authprofile.ChooseLoginDefault(cmd, name, current)
			if err != nil {
				return err
			}
			prefix := gerritProfiles.Prefix(name) + "."
			metadata := map[string]string{prefix + "username": username, prefix + "url": secrets.GerritEndpoint}
			if changeDefault {
				metadata["gerrit.default_profile"] = name
			}
			credential := config.Credential{Key: prefix + "password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: username}
			cmd.Println("Saving account credentials; unlock your password manager and approve access if prompted.")
			if err := credential.SaveWithProvider(password, insecureStorage, metadata, provider); err != nil {
				return err
			}
			cmd.Printf("Authenticated as %s on %s; saved profile %s\n", username, secrets.GerritEndpoint, name)
			if changeDefault {
				cmd.Printf("Default profile: %s\n", name)
			} else if current != "" {
				cmd.Printf("Default profile remains: %s\n", current)
			}

			return nil
		},
	}
	authprofile.AddDefaultFlag(cmd)
	cmd.Flags().StringVarP(&username, "username", "u", "", "Gerrit username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Gerrit HTTP password (prefer a hidden prompt or stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the password from stdin")
	cmd.MarkFlagsMutuallyExclusive("password", "password-stdin")
	authprofile.AddStorageFlags(cmd, &provider, &insecureStorage)
	return cmd
}
