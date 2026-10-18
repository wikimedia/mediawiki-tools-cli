package phabricator

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func newPhabricatorAuthLoginCmd(site *string) *cobra.Command {
	var username, token, endpoint string
	var tokenStdin, insecureStorage bool
	var provider string
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Login to Phabricator using an API token",
		Long:    "Save a validated Phabricator token with the selected credential provider. The default destination is Wikimedia Phabricator; --url explicitly selects another HTTPS endpoint. A project-local phab.cfg never chooses where a newly entered token is sent.",
		Example: "login\nlogin --username myname --token-stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "" {
				if err := mwconfig.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
			siteName := resolveAuthSite(*site)
			normalized, err := secrets.Endpoint(endpoint)
			if err != nil {
				return err
			}
			if tokenStdin {
				token, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(username) == "" {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --username")
				}
				if err := survey.AskOne(&survey.Input{Message: "Phabricator username:"}, &username, survey.WithValidator(survey.Required)); err != nil {
					return err
				}
			}
			username = strings.TrimSpace(username)
			if strings.TrimSpace(token) == "" {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --token-stdin or --token")
				}
				cmd.Printf("Create a token at %s/settings/user/%s/page/apitokens/\n", normalized, username)
				if err := survey.AskOne(&survey.Password{Message: "Phabricator API token:"}, &token, survey.WithValidator(survey.Required)); err != nil {
					return err
				}
			}
			token = strings.TrimSpace(token)
			if username == "" || token == "" {
				return fmt.Errorf("username and token are required")
			}
			cfg := &PhabConfig{SiteName: siteName, URL: normalized, Username: username, Key: token}
			whoami, err := validatePhabricatorCredentials(cmd, cfg)
			if err != nil {
				return fmt.Errorf("Phabricator credentials were not validated or saved: %w", err)
			}
			prefix := "phabricator.sites." + siteName + "."
			credential := mwconfig.Credential{Key: prefix + "key", Service: "phabricator", Endpoint: normalized, Account: username}
			if err := credential.SaveWithProvider(token, insecureStorage, map[string]string{
				"phabricator.default_site": siteName, prefix + "url": normalized, prefix + "username": username, prefix + "logged_out": "false",
			}, provider); err != nil {
				return err
			}
			cmd.Printf("Authenticated as %s; credentials saved for %s\n", whoami, normalized)
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Phabricator username")
	cmd.Flags().StringVarP(&token, "token", "t", "", "API token (prefer a hidden prompt or stdin)")
	cmd.Flags().StringVar(&endpoint, "url", defaultPhabricatorURL, "Explicit Phabricator HTTPS endpoint")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read the API token from stdin")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Store the token in the private plaintext config using insecure storage")
	cmd.MarkFlagsMutuallyExclusive("token", "token-stdin")
	cmd.Flags().StringVar(&provider, "provider", "", "Override the credential provider (keyring or keepassxc)")
	cmd.MarkFlagsMutuallyExclusive("provider", "insecure-storage")
	cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "keepassxc"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
