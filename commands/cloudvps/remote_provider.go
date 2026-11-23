package cloudvps

import (
	"context"
	"fmt"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/spf13/cobra"
)

func remoteCompute(cmd *cobra.Command, project string) (*gophercloud.ServiceClient, error) {
	selected, _ := cmd.Flags().GetString("profile")
	name, project, err := resolveCloudProfile(selected, project)
	if err != nil {
		return nil, err
	}
	provider, err := authenticatedProfileProvider(cmd.Context(), name, project)
	if err != nil {
		return nil, err
	}
	return openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
}

func RemoteInstances(cmd *cobra.Command, project string) ([]servers.Server, error) {
	compute, err := remoteCompute(cmd, project)
	if err != nil {
		return nil, err
	}
	pages, err := servers.List(compute, servers.ListOpts{}).AllPages(cmd.Context())
	if err != nil {
		return nil, err
	}
	return servers.ExtractServers(pages)
}

func CreateRemoteInstance(cmd *cobra.Command, project, name, image, flavor, network, key string, userData []byte) (string, error) {
	if !dnsLabel.MatchString(name) {
		return "", fmt.Errorf("Cloud VPS instance name must be a lowercase DNS label (up to 63 characters)")
	}
	compute, err := remoteCompute(cmd, project)
	if err != nil {
		return "", err
	}
	resources, err := resolveRemoteResources(cmd, compute, remoteResources{image: image, flavor: flavor, network: network})
	if err != nil {
		return "", err
	}
	server, err := createInstance(cmd.Context(), compute, servers.CreateOpts{
		Name: name, ImageRef: resources.image, FlavorRef: resources.flavor, Networks: []servers.Network{{UUID: resources.network}}, UserData: userData,
	}, key)
	if err != nil {
		return "", err
	}
	return server.ID, nil
}

const (
	defaultRemoteImage   = "debian-13.0-trixie"
	defaultRemoteFlavor  = "g4.cores2.ram4.disk20"
	defaultRemoteNetwork = "VXLAN/IPv6-dualstack"
)

type remoteResources struct {
	image, flavor, network string
}

func resolveRemoteResources(cmd *cobra.Command, compute *gophercloud.ServiceClient, selected remoteResources) (remoteResources, error) {
	ctx := cmd.Context()
	if selected.image == "" {
		available, err := listImages(ctx, compute.ProviderClient)
		if err != nil {
			return selected, fmt.Errorf("list Cloud VPS images: %w", err)
		}
		selected.image, err = namedRemoteResource(available, "image", defaultRemoteImage, func(image images.Image) (string, string) {
			return image.ID, image.Name
		})
		if err != nil {
			return selected, err
		}
		cmd.Printf("Cloud VPS image: %s\n", defaultRemoteImage)
	}
	if selected.flavor == "" {
		available, err := listFlavors(ctx, compute)
		if err != nil {
			return selected, fmt.Errorf("list Cloud VPS flavors: %w", err)
		}
		selected.flavor, err = namedRemoteResource(available, "flavor", defaultRemoteFlavor, func(flavor flavors.Flavor) (string, string) {
			return flavor.ID, flavor.Name
		})
		if err != nil {
			return selected, err
		}
		cmd.Printf("Cloud VPS flavor: %s\n", defaultRemoteFlavor)
	}
	if selected.network == "" {
		available, err := listNetworks(ctx, compute.ProviderClient)
		if err != nil {
			return selected, fmt.Errorf("list Cloud VPS networks: %w", err)
		}
		selected.network, err = namedRemoteResource(available, "network", defaultRemoteNetwork, func(network networks.Network) (string, string) {
			return network.ID, network.Name
		})
		if err != nil {
			return selected, err
		}
		cmd.Printf("Cloud VPS network: %s\n", defaultRemoteNetwork)
	}
	return selected, nil
}

func namedRemoteResource[T any](available []T, flag, name string, identify func(T) (id, name string)) (string, error) {
	var selected string
	for _, resource := range available {
		id, resourceName := identify(resource)
		if resourceName != name || id == "" {
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("multiple Cloud VPS %ss match %q; specify --%s with the resource ID", flag, name, flag)
		}
		selected = id
	}
	if selected == "" {
		return "", fmt.Errorf("default Cloud VPS %s %q is unavailable; use mw cloudvps %s list and pass --%s with a resource ID", flag, name, flag, flag)
	}
	return selected, nil
}

func createInstance(ctx context.Context, client *gophercloud.ServiceClient, opts servers.CreateOpts, key string) (*servers.Server, error) {
	return servers.Create(ctx, client, keypairs.CreateOptsExt{CreateOptsBuilder: opts, KeyName: key}, nil).Extract()
}

func WakeRemoteInstance(cmd *cobra.Command, project, id string, owned bool) error {
	compute, err := remoteCompute(cmd, project)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()
	getStatus := func(ctx context.Context) (string, error) {
		server, err := servers.Get(ctx, compute, id).Extract()
		if err != nil {
			return "", err
		}
		return server.Status, nil
	}
	status, err := getStatus(ctx)
	if err != nil || status == "ACTIVE" {
		return err
	}
	if !owned {
		return fmt.Errorf("existing Cloud VPS instance %s is %s; start it in Cloud VPS before attaching", id, status)
	}
	if status == "SHUTOFF" {
		if err := servers.Start(ctx, compute, id).ExtractErr(); err != nil {
			return err
		}
	}
	return waitForResource(ctx, "ACTIVE", getStatus)
}

func ChangeRemoteInstance(cmd *cobra.Command, project, id string, destroy bool) error {
	compute, err := remoteCompute(cmd, project)
	if err != nil {
		return err
	}
	if destroy {
		return servers.Delete(cmd.Context(), compute, id).ExtractErr()
	}
	server, err := servers.Get(cmd.Context(), compute, id).Extract()
	if err != nil || server.Status == "SHUTOFF" {
		return err
	}
	return servers.Stop(cmd.Context(), compute, id).ExtractErr()
}
