package cloudvps

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const authUrl = "https://openstack.eqiad1.wikimediacloud.org:25000/v3"

func NewCloudVPSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cloudvps",
		Aliases: []string{"vps"},
		GroupID: "service",
		Short:   "Interact with the Wikimedia Cloud VPS setup (WORK IN PROGRESS)",
		RunE:    nil,
		Hidden:  true, // for now, as WIP
	}

	cmd.AddCommand(NewComputeCmd())
	cmd.AddCommand(NewAuthCmd())

	return cmd
}

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

func NewComputeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compute",
		Short: "Manage compute resources",
	}

	cmd.AddCommand(NewComputeListCmd())

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
