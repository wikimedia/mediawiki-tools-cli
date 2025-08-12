package cloudvps

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewComputeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compute",
		Short: "Manage compute resources",
	}

	cmd.AddCommand(NewComputeListCmd())
	cmd.AddCommand(NewComputeGetCmd())
	cmd.AddCommand(NewComputeSshCmd())

	return cmd
}

func NewComputeListCmd() *cobra.Command {
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Name", "Status", "ID"},
			ProcessObjects: func(objects interface{}, table *output.Table) {
				objMap, ok := objects.(map[interface{}]interface{})
				if ok {
					for _, object := range objMap {
						typedObject, ok := object.(servers.Server)
						if !ok {
							continue
						}
						table.AddRowS(typedObject.Name, typedObject.Status, typedObject.ID)
					}
				}
			},
		},
		AckBinding: func(objects interface{}, ack *output.Ack) {
			objMap, ok := objects.(map[interface{}]interface{})
			if ok {
				for _, object := range objMap {
					typedObject, ok := object.(servers.Server)
					if !ok {
						continue
					}
					ack.AddItem(typedObject.Status, typedObject.Name+" ("+typedObject.Status+") @ "+typedObject.ID)
				}
			}
		},
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List compute resources",
		RunE: func(cmd *cobra.Command, args []string) error {
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
				panic(err)
			}

			computeClient, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{})
			if err != nil {
				panic(err)
			}

			// List all servers
			allPages, err := servers.List(computeClient, servers.ListOpts{}).AllPages(context.Background())
			if err != nil {
				panic(err)
			}

			allServers, err := servers.ExtractServers(allPages)
			if err != nil {
				panic(err)
			}

			objects := make(map[interface{}]interface{}, len(allServers))
			for key, server := range allServers {
				objects[key] = server
			}

			out.Print(cmd, objects)

			return nil
		},
	}

	out.AddFlags(cmd, output.TableType)
	cmd.Flags().String("project", "", "Project name (optional, uses default project if not specified)")

	return cmd
}
