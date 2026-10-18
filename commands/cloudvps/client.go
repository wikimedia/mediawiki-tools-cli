package cloudvps

import (
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

type providerFactory func(*cobra.Command) (*gophercloud.ProviderClient, error)

func projectName(cmd *cobra.Command) (string, error) {
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = config.State().Effective.CloudVPS.DefaultProject
	}
	if project == "" {
		return "", fmt.Errorf("project is required: use --project <name> or set cloud_vps.default_project")
	}
	return project, nil
}

func projectCredentials(project string, projects map[string]config.CloudVPSProject) (config.CloudVPSProjectCredential, error) {
	entry, exists := projects[project]
	if !exists || entry.Credential.ID == "" {
		return config.CloudVPSProjectCredential{}, fmt.Errorf("no application credentials configured for project %q; configure them with cloudvps auth add --project %s --id <application-credential-id> --secret-stdin", project, project)
	}
	if entry.Credential.Secret == "" {
		var err error
		entry.Credential.Secret, err = (config.Credential{Key: "cloud_vps.projects." + project + ".credential.secret", Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: project + ":" + entry.Credential.ID}).Read()
		if err != nil {
			return entry.Credential, err
		}
	}
	if entry.Credential.Secret == "" {
		return entry.Credential, fmt.Errorf("no application secret configured for project %q; use cloudvps auth add", project)
	}
	return entry.Credential, nil
}

func authenticatedProvider(cmd *cobra.Command) (*gophercloud.ProviderClient, error) {
	project, err := projectName(cmd)
	if err != nil {
		return nil, err
	}
	credentials, err := projectCredentials(project, config.State().Effective.CloudVPS.Projects)
	if err != nil {
		return nil, err
	}
	provider, err := openstack.AuthenticatedClient(cmd.Context(), gophercloud.AuthOptions{
		IdentityEndpoint:            authUrl,
		ApplicationCredentialID:     credentials.ID,
		ApplicationCredentialSecret: credentials.Secret,
		DomainID:                    "default",
		AllowReauth:                 true,
	})
	if err != nil {
		return nil, fmt.Errorf("authenticate to Cloud VPS project %q: %w", project, err)
	}
	return provider, nil
}

func addProjectFlag(cmd *cobra.Command) {
	cmd.Flags().String("project", "", "Project name (uses cloud_vps.default_project if omitted)")
}
