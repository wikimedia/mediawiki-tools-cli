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
		Use:   "ssh [name-or-id]",
		Short: "SSH to a compute resource by name or ID",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			id, _ := cmd.Flags().GetString("id")
			nameOrID := ""
			if len(args) > 0 {
				nameOrID = args[0]
			}

			// Validate that only one of name, id, or nameOrID is provided
			if (name != "" && id != "") || (name != "" && nameOrID != "") || (id != "" && nameOrID != "") {
				return fmt.Errorf("only one of --name, --id, or positional argument can be provided")
			}
			if name == "" && id == "" && nameOrID == "" {
				return fmt.Errorf("one of --name, --id, or positional argument must be provided")
			}

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
			var server *servers.Server

			if id != "" {
				server, err = servers.Get(context.Background(), computeClient, id).Extract()
			} else if name != "" {
				listOpts := servers.ListOpts{
					Name: name,
				}
				allPages, listErr := servers.List(computeClient, listOpts).AllPages(context.Background())
				if listErr != nil {
					return listErr
				}
				allServers, extractErr := servers.ExtractServers(allPages)
				if extractErr != nil {
					return extractErr
				}
				if len(allServers) == 0 {
					return fmt.Errorf("no server found with name: %s", name)
				}
				if len(allServers) > 1 {
					return fmt.Errorf("multiple servers found with name: %s, please use ID", name)
				}
				server = &allServers[0]
			} else if nameOrID != "" {
				server, err = resolveServerNameOrID(context.Background(), computeClient, nameOrID)
			}

			if err != nil {
				return err
			}

			hostname := fmt.Sprintf("%s.%s.eqiad1.wikimedia.cloud", server.Name, server.TenantID)
			logrus.Debugf("SSHing to %s", hostname)

			devConfig := c.Effective.Developer
			var sshCmd *exec.Cmd

			if devConfig.Username != "" {
				// Configured SSH
				logrus.Debugf("Using configured developer username %s and ssh key path %s", devConfig.Username, devConfig.SSHKeyPath)

				userHost := fmt.Sprintf("%s@%s", devConfig.Username, hostname)
				bastion := fmt.Sprintf("%s@bastion.wmcloud.org:22", devConfig.Username)

				args := []string{"-J", bastion, userHost}
				// only use the key if defined
				if devConfig.SSHKeyPath != "" {
					args = append([]string{"-i", devConfig.SSHKeyPath}, args...)
				}

				sshCmd = exec.Command("ssh", args...)

			} else {
				// Unconfigured SSH, rely on user's environment
				logrus.Debug("No developer username configured, using system ssh")
				sshCmd = exec.Command("ssh", hostname)
			}

			sshCmd.Stdin = os.Stdin
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr

			return sshCmd.Run()
		},
	}

	cmd.Flags().String("project", "", "Project name (optional, uses default project if not specified)")
	cmd.Flags().String("name", "", "Compute resource name")
	cmd.Flags().String("id", "", "Compute resource ID")

	return cmd
}
