package cloudvps

import (
	"context"
	"fmt"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/volumeattach"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewVolumeCmd() *cobra.Command {
	return newVolumeCmd(authenticatedProvider)
}

func newVolumeCmd(provider providerFactory) *cobra.Command {
	parent := &cobra.Command{Use: "volume", Short: "Create, inspect, and attach Cloud VPS block storage"}
	for _, action := range []string{"list", "get", "create", "attach"} {
		parent.AddCommand(newVolumeActionCmd(action, provider))
	}
	return parent
}

func newVolumeActionCmd(action string, provider providerFactory) *cobra.Command {
	var size int
	var serverID, zone string
	var wait bool
	var timeout time.Duration
	out := output.Output{}
	cmd := &cobra.Command{Use: action, Short: action + " Cloud VPS volumes", Args: cobra.ExactArgs(1)}
	switch action {
	case "list":
		cmd.Args = cobra.NoArgs
		cmd.Example = "list --project mwcli"
	case "get", "attach":
		cmd.Use += " <volume-id>"
		cmd.Example = action + " <volume-id> --project mwcli"
		if action == "attach" {
			cmd.Example += " --server <instance-id> --wait"
		}
	case "create":
		cmd.Use += " <name>"
		cmd.Example = "create ci-docker --project mwcli --size 40 --wait"
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if action == "create" && size <= 0 {
			return fmt.Errorf("--size must be a positive number of GiB")
		}
		if action == "attach" && serverID == "" {
			return fmt.Errorf("--server <instance-id> is required")
		}
		if timeout <= 0 {
			return fmt.Errorf("--timeout must be positive")
		}
		p, err := provider(cmd)
		if err != nil {
			return err
		}
		// Attaching uses Nova; Cinder access is only required when waiting.
		var client *gophercloud.ServiceClient
		if action != "attach" || wait {
			client, err = openstack.NewBlockStorageV3(p, gophercloud.EndpointOpts{})
			if err != nil {
				return err
			}
		}
		var result any
		var volumeID, target string
		switch action {
		case "list":
			pages, err := volumes.List(client, volumes.ListOpts{}).AllPages(cmd.Context())
			if err != nil {
				return err
			}
			result, err = volumes.ExtractVolumes(pages)
			if err != nil {
				return err
			}
		case "get":
			result, err = volumes.Get(cmd.Context(), client, args[0]).Extract()
			if err != nil {
				return err
			}
		case "create":
			volume, err := volumes.Create(cmd.Context(), client, volumes.CreateOpts{Name: args[0], Size: size, AvailabilityZone: zone}, nil).Extract()
			if err != nil {
				return fmt.Errorf("create volume: %w", err)
			}
			result, volumeID, target = volume, volume.ID, "available"
		case "attach":
			compute, err := openstack.NewComputeV2(p, gophercloud.EndpointOpts{})
			if err != nil {
				return err
			}
			result, err = volumeattach.Create(cmd.Context(), compute, serverID, volumeattach.CreateOpts{VolumeID: args[0]}).Extract()
			if err != nil {
				return fmt.Errorf("attach volume: %w", err)
			}
			volumeID, target = args[0], "in-use"
		}
		// Print the resource ID before waiting so interrupted operations can be inspected.
		out.Print(cmd, result)
		if wait && volumeID != "" {
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			if err := waitForResource(ctx, target, func(ctx context.Context) (string, error) {
				current, err := volumes.Get(ctx, client, volumeID).Extract()
				if err != nil {
					return "", err
				}
				return current.Status, nil
			}); err != nil {
				return fmt.Errorf("volume %s: %s request succeeded but waiting for %s failed: %w", volumeID, action, target, err)
			}
		}
		return nil
	}
	addProjectFlag(cmd)
	if action == "create" {
		cmd.Flags().IntVar(&size, "size", 0, "Volume size in GiB (required)")
		cmd.Flags().StringVar(&zone, "availability-zone", "", "Availability zone (must be compatible with the instance)")
	}
	if action == "attach" {
		cmd.Flags().StringVar(&serverID, "server", "", "Instance ID (required; attaches only, does not format or mount)")
	}
	if action == "create" || action == "attach" {
		cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the requested volume state")
		cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum time to wait for the requested state")
	} else {
		timeout = 10 * time.Minute
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType))
	return cmd
}
