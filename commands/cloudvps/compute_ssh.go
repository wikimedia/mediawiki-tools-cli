
package cloudvps

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewComputeSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh <id>",
		Short: "SSH to a compute resource by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				c := config.State()
				project = c.Effective.CloudVPS.DefaultProject
				if project == "" {
					return fmt.Errorf("project is required")
				}
			}

			c := config.State()
			// make sure the project exists
			_, exists := c.Effective.CloudVPS.Projects[project]
			if !exists {
				return fmt.Errorf("project not found: %s", project)
			}
			credentials := c.Effective.CloudVPS.Projects[project].Credential
			logrus.Tracef("Using credentials for project %s", project)

			auth := gophercloud.AuthOptions{
				IdentityEndpoint:            authUrl,
				ApplicationCredentialID:     credentials.ID,
				ApplicationCredentialSecret: credentials.Secret,
				DomainID:                    "default",
				AllowReauth:                 true,
			}

			providerClient, err := openstack.AuthenticatedClient(context.Background(), auth)
			if err != nil {
				return err
			}

			computeClient, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{})
			if err != nil {
				return err
			}

			server, err := servers.Get(context.Background(), computeClient, id).Extract()
			if err != nil {
				return err
			}

			hostname := fmt.Sprintf("%s.%s.eqiad1.wikimedia.cloud", server.Name, server.TenantID)
			logrus.Infof("SSHing to %s", hostname)

			sshCmd := exec.Command("ssh", hostname)
			sshCmd.Stdin = os.Stdin
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr

			return sshCmd.Run()
		},
	}

	cmd.Flags().String("project", "", "Project name (optional, uses default project if not specified)")

	return cmd
}
