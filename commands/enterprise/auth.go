package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/credentialcheck"
	api "gitlab.wikimedia.org/repos/releng/cli/internal/enterprise"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var profiles = authprofile.Manager{Service: "enterprise", Label: "Enterprise", SecretField: "token", Endpoint: api.API}

func credential(name string) (config.Credential, error) {
	name, err := profiles.Selected(name)
	if err != nil {
		return config.Credential{}, err
	}
	return config.CredentialForKey(profiles.Prefix(name) + ".token")
}

func client(name string) (*api.Client, error) {
	cred, err := credential(name)
	if err != nil {
		return nil, err
	}
	c := api.NewClient()
	c.Token = func(ctx context.Context, rejected string) (string, error) {
		value, err := cred.ReadContext(ctx)
		if err != nil {
			return "", err
		}
		tokens, err := api.ParseTokens(value)
		if err != nil {
			return "", err
		}
		if time.Until(tokens.ExpiresAt) > time.Minute && tokens.AccessToken != rejected {
			return tokens.AccessToken, nil
		}
		value, err = cred.Update(ctx, func(current string) (string, error) {
			tokens, err := api.ParseTokens(current)
			if err != nil {
				return "", err
			}
			if time.Until(tokens.ExpiresAt) > time.Minute && tokens.AccessToken != rejected {
				return current, nil
			}
			tokens, err = c.Refresh(ctx, cred.Account, tokens)
			if err != nil {
				return "", err
			}
			data, err := json.Marshal(tokens)
			return string(data), err
		})
		if err != nil {
			return "", err
		}
		tokens, err = api.ParseTokens(value)
		return tokens.AccessToken, err
	}
	return c, nil
}

func newAuthCmd(profile *string) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage Enterprise accounts and stored credentials"}
	shared := authprofile.Commands{Label: "Enterprise", Selected: profile, Backend: profiles}
	cmd.AddCommand(newLoginCmd(profile), shared.ProfilesCmd(), shared.DefaultCmd(), shared.RemoveCmd(), newStatusCmd(profile), newRevokeCmd(profile))
	return cmd
}

func newLoginCmd(profile *string) *cobra.Command {
	var username, provider string
	var stdin, insecure bool
	cmd := &cobra.Command{
		Use: "login", Short: "Sign in with an Enterprise username and password",
		Long:    "Exchange an Enterprise account password for access and refresh tokens. Only the tokens are saved, using the configured secret provider. Passwords are read from a hidden prompt or --password-stdin. Complete signup or required password changes at https://dashboard.enterprise.wikimedia.com first.",
		Example: "login\nlogin --profile bot --username mybot --password-stdin", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if provider != "" {
				if err := config.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
			if insecure && provider != "" {
				return fmt.Errorf("--insecure-storage and --provider cannot be combined")
			}
			if *profile != "" && !authprofile.ValidName(*profile) {
				return fmt.Errorf("invalid profile name")
			}
			username = strings.ToLower(strings.TrimSpace(username))
			if username == "" {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --username in non-interactive mode")
				}
				if err := survey.AskOne(&survey.Input{Message: "Enterprise username:"}, &username, survey.WithValidator(survey.Required)); err != nil {
					return err
				}
				username = strings.ToLower(strings.TrimSpace(username))
			}
			name := *profile
			if name == "" {
				name = username
			}
			if !authprofile.ValidName(name) {
				return fmt.Errorf("provide --profile using letters, digits, underscores or hyphens")
			}
			var password string
			var err error
			if stdin {
				password, err = secrets.ReadInput(cmd.InOrStdin())
			} else {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --password-stdin in non-interactive mode")
				}
				err = survey.AskOne(&survey.Password{Message: "Enterprise password:"}, &password, survey.WithValidator(survey.Required))
			}
			if err != nil {
				return err
			}
			tokens, err := api.NewClient().Login(cmd.Context(), username, password)
			if err != nil {
				return err
			}
			makeDefault, err := authprofile.ChooseLoginDefault(cmd, name, profiles.Default())
			if err != nil {
				return err
			}
			prefix := profiles.Prefix(name) + "."
			metadata := map[string]string{prefix + "username": username, prefix + "url": api.API}
			if makeDefault {
				metadata["enterprise.default_profile"] = name
			}
			data, err := json.Marshal(tokens)
			if err != nil {
				return err
			}
			cred := config.Credential{Key: prefix + "token", Service: "enterprise", Endpoint: api.API, Account: username}
			if config.HasCredential(cred.Key) {
				previous, loadErr := config.CredentialForKey(cred.Key)
				if loadErr != nil {
					return loadErr
				}
				err = cred.Replace(previous, string(data), insecure, metadata, provider)
			} else {
				err = cred.SaveWithProvider(string(data), insecure, metadata, provider)
			}
			if err != nil {
				return err
			}
			cmd.Printf("Saved Enterprise profile %s for %s.\n", name, username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Enterprise account username")
	cmd.Flags().BoolVar(&stdin, "password-stdin", false, "Read the password from standard input")
	authprofile.AddStorageFlags(cmd, &provider, &insecure)
	authprofile.AddDefaultFlag(cmd)
	return cmd
}

func newStatusCmd(profile *string) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use: "status", Short: "Show local account metadata; optionally check API access", Example: "status\nstatus --check", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cred, err := credential(*profile)
			if err != nil {
				return err
			}
			provider, err := cred.CredentialProvider("")
			if err != nil {
				return err
			}
			cmd.Printf("Username: %s\nEndpoint: %s\nProvider: %s\n", cred.Account, cred.Endpoint, provider)
			if !config.HasCredential(cred.Key) {
				return fmt.Errorf("profile has no saved credential; use enterprise auth login")
			}
			if !check {
				return nil
			}
			value, err := cred.ReadContext(cmd.Context())
			if err != nil {
				return err
			}
			result := credentialcheck.Check(cmd.Context(), credentialcheck.Request{Service: cred.Service, Endpoint: cred.Endpoint, Account: cred.Account, Value: value})
			cmd.Println("Access:", result.Status)
			if result.Status != "valid" {
				return fmt.Errorf("Enterprise access check: %s", result.Status)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Unlock the saved credential and check access without refreshing tokens")
	return cmd
}

func newRevokeCmd(profile *string) *cobra.Command {
	return &cobra.Command{
		Use: "revoke", Short: "Revoke the remote session and remove its local profile", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := profiles.Selected(*profile)
			if err != nil {
				return err
			}
			cred, err := credential(name)
			if err != nil {
				return err
			}
			// Hold the credential lock across revocation so a concurrent refresh
			// cannot restore usable tokens after the session has been revoked.
			_, err = cred.Update(cmd.Context(), func(value string) (string, error) {
				tokens, err := api.ParseTokens(value)
				if err != nil {
					return "", err
				}
				if err := api.NewClient().Revoke(cmd.Context(), tokens); err != nil {
					return "", err
				}
				return value, nil
			})
			if err != nil {
				return err
			}
			_, err = profiles.Remove(name)
			if err == nil {
				cmd.Println("Revoked Enterprise session and removed profile", name)
			}
			return err
		},
	}
}
