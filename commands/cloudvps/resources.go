package cloudvps

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func listImages(ctx context.Context, provider *gophercloud.ProviderClient) ([]images.Image, error) {
	client, err := openstack.NewImageV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}
	pages, err := images.List(client, images.ListOpts{Status: images.ImageStatusActive}).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return images.ExtractImages(pages)
}

func listFlavors(ctx context.Context, compute *gophercloud.ServiceClient) ([]flavors.Flavor, error) {
	pages, err := flavors.ListDetail(compute, flavors.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return flavors.ExtractFlavors(pages)
}

func listNetworks(ctx context.Context, provider *gophercloud.ProviderClient) ([]networks.Network, error) {
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}
	pages, err := networks.List(client, networks.ListOpts{}).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return networks.ExtractNetworks(pages)
}

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
				result, err = listImages(cmd.Context(), p)
			case "flavor":
				client, clientErr := openstack.NewComputeV2(p, gophercloud.EndpointOpts{})
				if clientErr != nil {
					return clientErr
				}
				result, err = listFlavors(cmd.Context(), client)
			case "network":
				result, err = listNetworks(cmd.Context(), p)
			}
			if err != nil {
				return err
			}
			return out.Print(cmd, result)
		},
	}
	addProjectFlag(cmd)
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType))
	parent.AddCommand(cmd)
	return parent
}
