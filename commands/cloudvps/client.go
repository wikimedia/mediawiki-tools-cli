package cloudvps

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/credentialcheck"
)

type providerFactory func(*cobra.Command) (*gophercloud.ProviderClient, error)

func projectName(cmd *cobra.Command) (string, error) {
	if profile, _ := cmd.Flags().GetString("profile"); profile != "" {
		_, project, err := selectedCloudProfile(cmd)
		return project, err
	}
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = config.State().Effective.CloudVPS.DefaultProject
	}
	if project == "" {
		_, selectedProject, err := selectedCloudProfile(cmd)
		return selectedProject, err
	}
	return project, nil
}

func authenticatedProvider(cmd *cobra.Command) (*gophercloud.ProviderClient, error) {
	name, project, err := selectedCloudProfile(cmd)
	if err != nil {
		return nil, err
	}
	return authenticatedProfileProvider(cmd.Context(), name, project)
}

func authenticatedProfileProvider(ctx context.Context, name, project string) (*gophercloud.ProviderClient, error) {
	credentials, err := profileCredentials(name)
	if err != nil {
		return nil, err
	}
	provider, err := authenticatedCloudClient(ctx, gophercloud.AuthOptions{
		IdentityEndpoint:            authUrl,
		ApplicationCredentialID:     credentials.ID,
		ApplicationCredentialSecret: credentials.Secret,
		DomainID:                    "default",
		AllowReauth:                 true,
	})
	if err != nil {
		return nil, fmt.Errorf("authenticate to Cloud VPS project %q: %w", project, err)
	}
	if err := checkProviderProject(provider, project); err != nil {
		return nil, err
	}
	return provider, nil
}

func addProjectFlag(cmd *cobra.Command) {
	cmd.Flags().String("project", "", "Project name (otherwise uses cloud_vps.default_project or the selected profile)")
}

// Configure identity before authentication so token requests and reauthentication
// carry it as well as compute, network, image and volume requests.
func authenticatedCloudClient(ctx context.Context, options gophercloud.AuthOptions) (*gophercloud.ProviderClient, error) {
	provider, err := openstack.NewClient(options.IdentityEndpoint)
	if err != nil {
		return nil, err
	}
	provider.UserAgent.Prepend(cli.HTTPUserAgent)
	if err := openstack.Authenticate(ctx, provider, options); err != nil {
		return nil, err
	}
	return provider, nil
}

func checkProviderProject(provider *gophercloud.ProviderClient, expected string) error {
	result, ok := provider.GetAuthResult().(tokens.CreateResult)
	if !ok {
		return fmt.Errorf("could not verify the authenticated Cloud VPS project")
	}
	project, err := result.ExtractProject()
	if err != nil || project == nil {
		return fmt.Errorf("could not read the authenticated Cloud VPS project")
	}
	username := ""
	if user, err := result.ExtractUser(); err == nil && user != nil {
		username = user.Name
	}
	checked := credentialcheck.CloudProjectResult(expected, project.ID, project.Name, username)
	if checked.Status != "valid" {
		return fmt.Errorf("Cloud VPS project verification failed: %s", checked.Detail)
	}
	return nil
}
