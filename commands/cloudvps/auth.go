package cloudvps

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

const authUrl = secrets.CloudEndpoint

var loginCloudClient = authenticatedCloudClient

func NewAuthCmd(selected *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Cloud VPS",
	}

	profiles := authprofile.Commands{Label: "Cloud VPS", Selected: selected, Backend: cloudProfiles}
	cmd.AddCommand(NewAuthLoginCmd(), NewAuthStatusCmd(), profiles.RemoveCmd(), profiles.ProfilesCmd(), profiles.DefaultCmd())

	return cmd
}

func NewAuthLoginCmd() *cobra.Command {
	var replace bool
	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"add"},
		Short:   "Add a new application credential",
		Long:    "Validate and save OpenStack application credentials. Missing values are prompted for in a terminal, with the secret hidden. Create credentials for the selected project at https://horizon.wikimedia.org/identity/application_credentials/. Use --replace to rotate an existing profile to a new credential for the same project. The destination is verified and saved before the old local entry is cleaned up; server-side credentials are not revoked.",
		Example: "login --project mwcli\nlogin --project mwcli --profile mwcli-readonly --id APPLICATION_CREDENTIAL_ID --secret-stdin",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			if name, _ := cmd.Flags().GetString("profile"); name != "" && !authprofile.ValidName(name) {
				return fmt.Errorf("profile must contain only letters, digits, underscores or hyphens")
			}
			id, _ := cmd.Flags().GetString("id")
			secret, _ := cmd.Flags().GetString("secret")
			secretStdin, _ := cmd.Flags().GetBool("secret-stdin")
			insecureStorage, _ := cmd.Flags().GetBool("insecure-storage")
			provider, _ := cmd.Flags().GetString("provider")
			if provider != "" {
				if err := config.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
			if secretStdin {
				var err error
				secret, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}

			if project == "" && !cmd.Flags().Changed("project") {
				name, _ := cmd.Flags().GetString("profile")
				if name != "" {
					project = config.State().EffectiveKoanf.String(cloudProfiles.Prefix(name) + ".project")
				}
				if project == "" {
					project = config.State().Effective.CloudVPS.DefaultProject
				}
			}
			if project == "" || id == "" || secret == "" {
				if !cli.CanAskForInput() {
					return fmt.Errorf("provide --project (or cloud_vps.default_project), --id and --secret; run in an interactive terminal to be prompted; create credentials at https://horizon.wikimedia.org/identity/application_credentials/")
				}
				if err := promptApplicationCredential(cmd, &project, &id, &secret, survey.AskOne); err != nil {
					return err
				}
			}
			if !sshDNSLabel.MatchString(project) {
				return fmt.Errorf("project must be a valid Cloud VPS project name")
			}

			auth := gophercloud.AuthOptions{
				IdentityEndpoint:            authUrl,
				ApplicationCredentialID:     id,
				ApplicationCredentialSecret: secret,
				DomainID:                    "default",
			}
			cloudClient, err := loginCloudClient(cmd.Context(), auth)
			if err != nil {
				return err
			}

			if err := checkProviderProject(cloudClient, project); err != nil {
				return err
			}

			name, _ := cmd.Flags().GetString("profile")
			if name == "" {
				name = project
			}
			if !authprofile.ValidName(name) {
				return fmt.Errorf("profile must contain only letters, digits, underscores or hyphens")
			}
			changeDefault := false
			if !replace || cmd.Flags().Changed("default") {
				changeDefault, err = authprofile.ChooseLoginDefault(cmd, name, cloudProfiles.Default())
				if err != nil {
					return err
				}
			}
			key := cloudProfiles.Prefix(name) + "."
			metadata := map[string]string{key + "id": id, key + "project": project}
			if changeDefault {
				metadata["cloud_vps.default_profile"] = name
			}
			credential := config.Credential{Key: key + "secret", Service: "cloudvps", Endpoint: authUrl, Account: project + ":" + id}
			previousID := config.State().EffectiveKoanf.String(key + "id")
			previousProject := config.State().EffectiveKoanf.String(key + "project")
			if previousProject != "" && previousProject != project {
				return fmt.Errorf("profile %q belongs to project %q; use a separate profile for %q", name, previousProject, project)
			}
			if replace && previousID == "" {
				return fmt.Errorf("profile %q does not exist; omit --replace to create it", name)
			}
			if previousID != "" && previousID != id && !replace {
				return fmt.Errorf("profile %q already has another application credential; use --replace to rotate it", name)
			}
			cmd.Println("Saving account credentials; unlock your password manager and approve access if prompted.")
			if replace {
				previous := credential
				previous.Account = previousProject + ":" + previousID
				if err := credential.Replace(previous, secret, insecureStorage, metadata, provider); err != nil {
					return err
				}
			} else if err := credential.SaveWithProvider(secret, insecureStorage, metadata, provider); err != nil {
				return err
			}

			cmd.Printf("Saved Cloud VPS profile %s for project %s.\n", name, project)
			if changeDefault {
				cmd.Printf("Default profile: %s\n", name)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&replace, "replace", false, "Replace the existing profile's application credential for the same project")
	cmd.Flags().String("project", "", "Project name (uses cloud_vps.default_project, otherwise prompts)")
	cmd.Flags().String("id", "", "Application credential ID (prompts if omitted)")
	cmd.Flags().String("secret", "", "Application credential secret (prompts with hidden input if omitted)")

	cmd.Flags().Bool("secret-stdin", false, "Read the application secret from stdin")
	cmd.MarkFlagsMutuallyExclusive("secret", "secret-stdin")
	var provider string
	var insecure bool
	authprofile.AddStorageFlags(cmd, &provider, &insecure)
	authprofile.AddDefaultFlag(cmd)

	return cmd
}

func promptApplicationCredential(cmd *cobra.Command, project, id, secret *string, ask func(survey.Prompt, interface{}, ...survey.AskOpt) error) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "Select your project in Horizon, then create an application credential:")
	fmt.Fprintln(cmd.ErrOrStderr(), "https://horizon.wikimedia.org/identity/application_credentials/")
	if *project == "" {
		if err := ask(projectPrompt(cmd.Context()), project, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}
	if *id == "" {
		if err := ask(&survey.Input{Message: "Application credential ID:"}, id, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}
	if *secret == "" {
		if err := ask(&survey.Password{Message: "Application credential secret:"}, secret, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}
	return nil
}

func NewAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "status", Aliases: []string{"check"},
		Short: "Check the selected Cloud VPS application credential and project scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, project, err := selectedCloudProfile(cmd)
			if err != nil {
				return err
			}
			if _, err := authenticatedProvider(cmd); err != nil {
				return err
			}
			cmd.Printf("Cloud VPS profile %s is valid for project %s.\n", name, project)
			return nil
		},
	}
	addProjectFlag(cmd)
	return cmd
}
