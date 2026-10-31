package gitlab

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/mwcli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/credentialcheck"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func newGitlabAuthCmd(session *profileSession) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage saved GitLab account profiles"}
	profiles := authprofile.Commands{Label: "GitLab", Selected: session.selected, Backend: gitlabProfiles}
	cmd.AddCommand(newGitlabLoginCmd(session), newGitlabStatusCmd(session), profiles.ProfilesCmd(), profiles.DefaultCmd(), profiles.RemoveCmd(), newGitCredentialCmd(session))
	return cmd
}

func newGitlabLoginCmd(session *profileSession) *cobra.Command {
	var hostname, token string
	var stdin, fromGlab, insecure bool
	var provider string
	cmd := &cobra.Command{Use: "login", Short: "Validate and save a GitLab personal access token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if provider != "" {
			if err := config.ValidateCredentialProvider(provider); err != nil {
				return err
			}
		}
		if name := strings.TrimSpace(*session.selected); name != "" && !authprofile.ValidName(name) {
			return fmt.Errorf("profile must contain only letters, digits, underscores or hyphens")
		}
		endpoint, err := normalizeGitlabEndpoint(hostname)
		if err != nil {
			return err
		}
		if stdin {
			token, err = secrets.ReadInput(cmd.InOrStdin())
			if err != nil {
				return err
			}
		}
		if fromGlab {
			host, _ := url.Parse(endpoint)
			cmd.Println("Reading the saved glab token for", host.Host)
			token, err = mwcli.LegacyToken(host.Host)
			if err != nil {
				return err
			}
		}
		if token == "" {
			if !cli.CanAskForInput() {
				return fmt.Errorf("provide --token-stdin or --from-glab, or run interactively for a hidden token prompt")
			}
			cmd.Println("Create a personal access token in your GitLab account settings. Choose read_api for read-only operations, or api for API writes; Git over HTTPS needs repository permissions too.")
			if err := survey.AskOne(&survey.Password{Message: "GitLab access token:"}, &token, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
		cmd.Println("Checking the authenticated GitLab identity...")
		checked := credentialcheck.GitLabIdentity(cmd.Context(), endpoint, token)
		if checked.Status != "valid" {
			return fmt.Errorf("GitLab credentials were not saved: %s", checked.Status)
		}
		name := strings.TrimSpace(*session.selected)
		if name == "" {
			name = strings.ToLower(checked.Identity)
		}
		if !authprofile.ValidName(name) {
			return fmt.Errorf("verified username cannot be used as a profile name; provide --profile using letters, digits, underscores or hyphens")
		}
		changeDefault, err := authprofile.ChooseLoginDefault(cmd, name, gitlabProfiles.Default())
		if err != nil {
			return err
		}
		prefix := gitlabProfiles.Prefix(name) + "."
		metadata := map[string]string{prefix + "username": checked.Identity, prefix + "url": endpoint}
		if changeDefault {
			metadata["gitlab.default_profile"] = name
		}
		credential := config.Credential{Key: prefix + "token", Service: "gitlab", Endpoint: endpoint, Account: checked.Identity}
		cmd.Println("Saving account credentials; unlock your password manager and approve access if prompted.")
		if err := credential.SaveWithProvider(token, insecure, metadata, provider); err != nil {
			return err
		}
		cmd.Printf("Authenticated as %s on %s; saved profile %s\n", checked.Identity, endpoint, name)
		if changeDefault {
			cmd.Printf("Default profile: %s\n", name)
		}
		if fromGlab {
			cmd.Println("Imported a copy. The original glab credential was retained.")
		}
		return nil
	}}
	cmd.Flags().StringVar(&hostname, "hostname", "gitlab.wikimedia.org", "GitLab hostname (HTTPS)")
	cmd.Flags().StringVar(&token, "token", "", "Personal access token (prefer a hidden prompt or --token-stdin)")
	cmd.Flags().BoolVar(&stdin, "token-stdin", false, "Read the token from stdin")
	cmd.Flags().BoolVar(&fromGlab, "from-glab", false, "Import and validate the host's saved glab token; retain the original")
	cmd.MarkFlagsMutuallyExclusive("token", "token-stdin", "from-glab")
	authprofile.AddStorageFlags(cmd, &provider, &insecure)
	authprofile.AddDefaultFlag(cmd)
	return cmd
}

func newGitlabStatusCmd(session *profileSession) *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Check the selected GitLab profile's identity", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		name, err := session.name()
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("no saved GitLab profile; use auth login, or auth login --from-glab to import an existing token")
		}
		profile, err := session.credentials()
		if err != nil {
			return err
		}
		cmd.Println("Reading saved GitLab credentials; unlock your password manager and approve access if prompted.")
		token, err := profile.Token()
		if err != nil {
			return err
		}
		checked := credentialcheck.Check(cmd.Context(), credentialcheck.Request{Service: "gitlab", Endpoint: "https://" + profile.Host, Account: profile.Username, Value: token})
		if checked.Status != "valid" {
			if checked.Status == "wrong-account" {
				return fmt.Errorf("authenticated as %s, expected %s", checked.Identity, profile.Username)
			}
			return fmt.Errorf("GitLab authentication check: %s", checked.Status)
		}
		cmd.Printf("Authenticated as %s on https://%s using profile %s.\n", checked.Identity, profile.Host, name)
		return nil
	}}
}
