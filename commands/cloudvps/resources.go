package cloudvps

import (
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newResourceListCmd(resource string, provider providerFactory) *cobra.Command {
	parent := &cobra.Command{Use: resource, Short: "Inspect available " + resource + " resources"}
	if resource == "flavor" {
		parent.Aliases = []string{"flavour"}
	}
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "list", Short: "List available " + resource + " resources and their IDs", Args: cobra.NoArgs,
		Example: "list --project mwcli",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := provider(cmd)
			if err != nil {
				return err
			}
			var result any
			switch resource {
			case "image":
				client, err := openstack.NewImageV2(p, gophercloud.EndpointOpts{})
				if err != nil {
					return err
				}
				pages, err := images.List(client, images.ListOpts{Status: images.ImageStatusActive}).AllPages(cmd.Context())
				if err != nil {
					return err
				}
				result, err = images.ExtractImages(pages)
				if err != nil {
					return err
				}
			case "flavor":
				client, err := openstack.NewComputeV2(p, gophercloud.EndpointOpts{})
				if err != nil {
					return err
				}
				pages, err := flavors.ListDetail(client, flavors.ListOpts{}).AllPages(cmd.Context())
				if err != nil {
					return err
				}
				result, err = flavors.ExtractFlavors(pages)
				if err != nil {
					return err
				}
			case "network":
				client, err := openstack.NewNetworkV2(p, gophercloud.EndpointOpts{})
				if err != nil {
					return err
				}
				pages, err := networks.List(client, networks.ListOpts{}).AllPages(cmd.Context())
				if err != nil {
					return err
				}
				result, err = networks.ExtractNetworks(pages)
				if err != nil {
					return err
				}
			}
			out.Print(cmd, result)
			return nil
		},
	}
	addProjectFlag(cmd)
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType))
	parent.AddCommand(cmd)
	return parent
}
