package cloudvps

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

// resolveServerNameOrID tries to find a server by ID, and if that fails, by name.
func resolveServerNameOrID(ctx context.Context, client *gophercloud.ServiceClient, nameOrID string) (*servers.Server, error) {
	// Try to get by ID first. We can't really know if the user provided a name or an ID.
	// We first try by ID, and if it fails with a 404, we try by name.
	server, err := servers.Get(ctx, client, nameOrID).Extract()
	if err == nil {
		return server, nil
	}

	// If we are here, it means we couldn't find the server by ID.
	// Let's try to find it by name.
	listOpts := servers.ListOpts{
		Name: nameOrID,
	}
	allPages, err := servers.List(client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, err
	}

	if len(allServers) == 0 {
		return nil, fmt.Errorf("server with name or ID '%s' not found", nameOrID)
	}

	if len(allServers) > 1 {
		return nil, fmt.Errorf("multiple servers found with name '%s', please use ID", nameOrID)
	}

	return &allServers[0], nil
}
