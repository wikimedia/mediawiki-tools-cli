package cloudvps

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const authUrl = secrets.CloudEndpoint

func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Cloud VPS",
	}

	cmd.AddCommand(NewAuthAddCmd())
	cmd.AddCommand(NewAuthRemoveCmd())
	cmd.AddCommand(NewAuthCheckCmd())
	cmd.AddCommand(NewAuthListCmd())

	return cmd
}

func NewAuthAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a new application credential",
		Long:    "Validate and save OpenStack application credentials. Missing values are prompted for in a terminal, with the secret hidden. Create credentials for the selected project at https://horizon.wikimedia.org/identity/application_credentials/.",
		Example: "add\nadd --project mwcli\nadd --project mwcli --id APPLICATION_CREDENTIAL_ID --secret-stdin",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetString("id")
			secret, _ := cmd.Flags().GetString("secret")
			secretStdin, _ := cmd.Flags().GetBool("secret-stdin")
			insecureStorage, _ := cmd.Flags().GetBool("insecure-storage")
			if secretStdin {
				var err error
				secret, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			}

			if project == "" && !cmd.Flags().Changed("project") {
				project = config.State().Effective.CloudVPS.DefaultProject
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
			_, err := openstack.AuthenticatedClient(cmd.Context(), auth)
			if err != nil {
				return err
			}

			// TODO check the creds are for the right project before saving?

			key := "cloud_vps.projects." + project + ".credential."
			credential := config.Credential{Key: key + "secret", Service: "cloudvps", Endpoint: authUrl, Account: project + ":" + id}
			if err := credential.SaveWithMetadata(secret, insecureStorage, map[string]string{key + "id": id}); err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), cmdgloss.SuccessHeading(fmt.Sprintf("Added new application credential for project: %s", project)))
			return nil
		},
	}

	cmd.Flags().String("project", "", "Project name (uses cloud_vps.default_project, otherwise prompts)")
	cmd.Flags().String("id", "", "Application credential ID (prompts if omitted)")
	cmd.Flags().String("secret", "", "Application credential secret (prompts with hidden input if omitted)")

	cmd.Flags().Bool("secret-stdin", false, "Read the application secret from stdin")
	cmd.Flags().Bool("insecure-storage", false, "Store the secret in the private plaintext config using insecure storage")
	cmd.MarkFlagsMutuallyExclusive("secret", "secret-stdin")
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

func NewAuthRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove application credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")

			if project == "" {
				return fmt.Errorf("project is required")
			}

			c := config.State()
			// make sure the project exists
			_, exists := c.Effective.CloudVPS.Projects[project]
			if !exists {
				return fmt.Errorf("project not found: %s", project)
			}

			credential, err := config.CredentialForKey("cloud_vps.projects." + project + ".credential.secret")
			if err != nil {
				return err
			}
			if err := credential.Delete(); err != nil {
				return err
			}
			if err := config.DeleteKeyValueFromDisk("cloud_vps.projects." + project + ".credential.id"); err != nil {
				return err
			}

			fmt.Print(cmdgloss.SuccessHeading(fmt.Sprintf("Removed application credential for project: %s", project)))
			return nil
		},
	}

	cmd.Flags().String("project", "", "Project name (required)")

	return cmd
}

func NewAuthCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check if the application credential is valid",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := projectName(cmd)
			if err != nil {
				return err
			}
			if _, err := authenticatedProvider(cmd); err != nil {
				return err
			}

			fmt.Print(cmdgloss.SuccessHeading(fmt.Sprintf("Credentials for project %s are valid", project)))
			return nil
		},
	}

	cmd.Flags().String("project", "", "Project name (required)")

	return cmd
}

func NewAuthListCmd() *cobra.Command {
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Project", "Credential ID"},
			ProcessObjects: func(objects interface{}, table *output.Table) {
				objMap, ok := objects.(map[string]interface{})
				if ok {
					for name, object := range objMap {
						cred, ok := object.(string)
						if !ok {
							cred = "(no credential ID)"
						}
						table.AddRowS(name, cred)
					}
				}
			},
		},
		PrettyBinding: func(objects interface{}, pretty *output.Pretty) {
			objMap, ok := objects.(map[string]interface{})
			if ok {
				for name, object := range objMap {
					cred, ok := object.(string)
					if !ok {
						cred = "(no credential ID)"
					}
					pretty.AddItem("Configured projects", name+": "+cred)
				}
			}
		},
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured Cloud VPS projects and their credential IDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := config.State()
			projects := c.Effective.CloudVPS.Projects
			if len(projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No Cloud VPS projects configured.")
				return nil
			}
			objects := make(map[string]interface{}, len(projects))
			for name, proj := range projects {
				id := proj.Credential.ID
				if id == "" {
					id = "(no credential ID)"
				}
				objects[name] = id
			}
			out.Print(cmd, objects)
			return nil
		},
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.PrettyType), output.WithDefaultPipe(output.JSONType))
	return cmd
}
