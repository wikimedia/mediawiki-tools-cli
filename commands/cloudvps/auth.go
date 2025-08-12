package cloudvps

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

const authUrl = "https://openstack.eqiad1.wikimediacloud.org:25000/v3"

func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "auth",
		Short:  "Authenticate with Cloud VPS",
		Hidden: true, // for now
	}

	cmd.AddCommand(NewAuthAddCmd())
	cmd.AddCommand(NewAuthRemoveCmd())
	cmd.AddCommand(NewAuthCheckCmd()) // Add the check command

	return cmd
}

func NewAuthAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new application credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			id, _ := cmd.Flags().GetString("id")
			secret, _ := cmd.Flags().GetString("secret")

			if project == "" || id == "" || secret == "" {
				// TODO interactive mode
				return fmt.Errorf("project, credential-id, and credential-secret are required")
			}

			auth := gophercloud.AuthOptions{
				IdentityEndpoint:            authUrl,
				ApplicationCredentialID:     id,
				ApplicationCredentialSecret: secret,
				DomainID:                    "default",
			}
			_, err := openstack.AuthenticatedClient(context.Background(), auth)
			if err != nil {
				return err
			}

			// TODO check the creds are for the right project before saving?

			config.PutKeyValueOnDisk("cloud_vps.projects."+project+".credential.id", id)
			config.PutKeyValueOnDisk("cloud_vps.projects."+project+".credential.secret", secret)

			fmt.Print(cmdgloss.SuccessHeading(fmt.Sprintf("Added new application credential for project: %s", project)))
			return nil
		},
	}

	cmd.Flags().String("project", "", "Project name (required)")
	cmd.Flags().String("id", "", "Application credential ID (required)")
	cmd.Flags().String("secret", "", "Application credential secret (required)")

	return cmd
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

			config.DeleteKeyValueFromDisk("cloud_vps.projects." + project + ".credential.id")
			config.DeleteKeyValueFromDisk("cloud_vps.projects." + project + ".credential.secret")

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
			project, _ := cmd.Flags().GetString("project")

			if project == "" {
				return fmt.Errorf("project is required")
			}

			c := config.State()
			// make sure the project exists
			credentials, exists := c.Effective.CloudVPS.Projects[project]
			if !exists {
				return fmt.Errorf("project not found: %s", project)
			}

			auth := gophercloud.AuthOptions{
				IdentityEndpoint:            authUrl,
				ApplicationCredentialID:     credentials.Credential.ID,
				ApplicationCredentialSecret: credentials.Credential.Secret,
				DomainID:                    "default",
			}

			_, err := openstack.AuthenticatedClient(context.Background(), auth)
			if err != nil {
				return fmt.Errorf("failed to authenticate: %v", err)
			}

			fmt.Print(cmdgloss.SuccessHeading(fmt.Sprintf("Credentials for project %s are valid", project)))
			return nil
		},
	}

	cmd.Flags().String("project", "", "Project name (required)")

	return cmd
}
