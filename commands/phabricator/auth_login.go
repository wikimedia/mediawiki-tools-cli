package phabricator

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/browser"
)

type loginInteraction struct {
	openURL        func(string) error
	readToken      func() (string, error)
	canAsk         func() bool
	confirmDefault func(profile, current string) (bool, error)
}

func newPhabricatorAuthLoginCmd(site *string) *cobra.Command {
	return newPhabricatorAuthLoginCmdWithInteraction(site, loginInteraction{
		openURL:        browser.OpenURL,
		canAsk:         cli.CanAskForInput,
		confirmDefault: authprofile.ConfirmDefault,
		readToken: func() (string, error) {
			var value string
			err := survey.AskOne(&survey.Password{Message: "Phabricator API token:"}, &value, survey.WithValidator(survey.Required))
			return value, err
		},
	})
}

func newPhabricatorAuthLoginCmdWithInteraction(site *string, interaction loginInteraction) *cobra.Command {
	var username, token, endpoint string
	var tokenStdin, insecureStorage bool
	var provider, method, configFile, configSection string
	var noBrowser bool
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Login to Phabricator through the browser or an existing API token",
		Long:    "Open Phabricator's browser login and paste its token into a hidden prompt. Verifying the token activates ongoing access until revoked. Use --method token for an existing API token, or --method phab-cfg to validate and copy credentials from an existing file. Save separate accounts with --profile; otherwise use the verified username in lowercase. The first login becomes the default; later logins ask before switching it. Use --default to switch explicitly. The default destination is Wikimedia Phabricator; --url explicitly selects another HTTPS endpoint. A project-local phab.cfg never chooses where a newly entered token is sent.",
		Example: "login --profile personal\nlogin --profile bot --method token\nlogin --profile bot --token-stdin",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "" {
				if err := mwconfig.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
			siteName := strings.TrimSpace(*site)
			if siteName != "" && !authprofile.ValidName(siteName) {
				return fmt.Errorf("profile must contain only letters, digits, underscores or hyphens")
			}
			if method != "browser" && method != "token" && method != "phab-cfg" {
				return fmt.Errorf("--method must be browser, token or phab-cfg")
			}
			normalized, err := secrets.Endpoint(endpoint)
			if err != nil {
				return err
			}
			var imported *PhabConfig
			if method == "phab-cfg" {
				if cmd.Flags().Changed("token") || tokenStdin {
					return fmt.Errorf("phab-cfg import cannot be combined with --token or --token-stdin")
				}
				imported, err = importPhabConfig(configFile, configSection)
				if err != nil {
					return err
				}
				if cmd.Flags().Changed("url") && normalized != imported.URL {
					return fmt.Errorf("--url must match the imported token's configured endpoint")
				}
				normalized, token = imported.URL, imported.Key
				if strings.TrimSpace(username) == "" {
					username = imported.Username
				}
				cmd.Printf("Checking phab.cfg section %s on %s before copying its credentials...\n", imported.SiteName, normalized)
			} else {
				if configFile != "" || configSection != "" {
					return fmt.Errorf("--config-file and --config-section require --method phab-cfg")
				}
				if !cmd.Flags().Changed("method") && !cmd.Flags().Changed("token") && !tokenStdin {
					if path := detectPhabConfig(); path != "" {
						cmd.Printf("Found %s. You can import and validate it with auth login --method phab-cfg --profile <name>.\n", path)
					}
				}
			}

			if tokenStdin {
				token, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}
			username = strings.TrimSpace(username)
			if strings.TrimSpace(token) == "" {
				if !interaction.canAsk() {
					return fmt.Errorf("provide --token-stdin or --token in non-interactive mode")
				}
				if method == "browser" {
					loginURL := normalized + "/conduit/login/"
					cmd.Printf("Open this URL to sign in to Phabricator:\n%s\n", loginURL)
					cmd.Println("Paste the token within one hour. Verification activates ongoing access until you revoke it.")
					if !noBrowser {
						if err := interaction.openURL(loginURL); err != nil {
							cmd.Println("Could not open the browser; open the URL above manually.")
						}
					}
				} else {
					cmd.Printf("Use an existing API token, or create one at %s/settings/panel/apitokens/\n", normalized)
					label := siteName
					if label == "" {
						label = username
					}
					if label != "" {
						cmd.Println("Suggested token name: a-wikimedia-cli — " + label)
					} else {
						cmd.Println("Suggested token name: a-wikimedia-cli")
					}
				}
				token, err = interaction.readToken()
				if err != nil {
					return err
				}
			}
			token = strings.TrimSpace(token)
			if token == "" {
				return fmt.Errorf("token is required")
			}
			cmd.Println("Checking the authenticated Phabricator identity...")
			cfg := &PhabConfig{SiteName: siteName, URL: normalized, Username: username, Key: token}
			whoami, err := validatePhabricatorCredentials(cmd, cfg)
			if err != nil {
				return fmt.Errorf("Phabricator credentials were not validated or saved: %w", err)
			}
			if username != "" && !strings.EqualFold(username, whoami) {
				return fmt.Errorf("token belongs to %s, expected %s; credentials were not saved", whoami, username)
			}
			username = whoami
			if siteName == "" {
				siteName = strings.ToLower(whoami)
				if !authprofile.ValidName(siteName) {
					return fmt.Errorf("provide --profile with letters, digits, underscores or hyphens for this account")
				}
			}
			currentDefault := phabProfiles.Default()
			changeDefault, err := authprofile.ChooseDefault(cmd, siteName, currentDefault, interaction.canAsk, interaction.confirmDefault)
			if err != nil {
				return err
			}
			prefix := "phabricator.profiles." + siteName + "."
			updates := map[string]string{
				prefix + "url":        normalized,
				prefix + "username":   username,
				prefix + "logged_out": "false",
			}
			if imported != nil && imported.DefaultProject != "" {
				updates[prefix+"default_project"] = imported.DefaultProject
			}
			if changeDefault {
				updates["phabricator.default_profile"] = siteName
			} else if currentDefault != "" {
				updates["phabricator.default_profile"] = currentDefault
			}
			credential := mwconfig.Credential{Key: prefix + "key", Service: "phabricator", Endpoint: normalized, Account: username}
			cmd.Println("Saving account credentials; unlock your password manager and approve access if prompted.")
			if err := credential.SaveWithProvider(token, insecureStorage, updates, provider); err != nil {
				return err
			}
			cmd.Printf("Authenticated as %s on %s; saved profile %s\n", whoami, normalized, siteName)
			if changeDefault {
				cmd.Printf("Default profile: %s\n", siteName)
			} else if currentDefault != "" {
				cmd.Printf("Default profile remains: %s\n", currentDefault)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Expected Phabricator username (otherwise discovered from the token)")
	cmd.Flags().StringVarP(&token, "token", "t", "", "API token (prefer a hidden prompt or stdin)")
	cmd.Flags().StringVar(&endpoint, "url", defaultPhabricatorURL, "Explicit Phabricator HTTPS endpoint")
	cmd.Flags().StringVar(&method, "method", "browser", "Login method: browser, token or phab-cfg (supplied tokens skip browser launch)")
	authprofile.AddDefaultFlag(cmd)
	cmd.Flags().StringVar(&configFile, "config-file", "", "phab.cfg file to import (otherwise search standard locations)")
	cmd.Flags().StringVar(&configSection, "config-section", "", "Section to import from phab.cfg (otherwise its [main] default)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the login URL without opening the browser")
	cmd.RegisterFlagCompletionFunc("method", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"browser", "token", "phab-cfg"}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read the API token from stdin")
	cmd.MarkFlagsMutuallyExclusive("token", "token-stdin")
	authprofile.AddStorageFlags(cmd, &provider, &insecureStorage)
	return cmd
}
