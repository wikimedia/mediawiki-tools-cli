package cloudvps

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

var dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NewComputeCreateCmd() *cobra.Command {
	return newComputeCreateCmd(authenticatedProvider)
}

func newComputeCreateCmd(provider providerFactory) *cobra.Command {
	var image, flavor, network, keyName, userData, zone string
	var groups []string
	var wait bool
	var timeout time.Duration
	out := output.Output{}
	cmd := &cobra.Command{
		Use:     "create <name>",
		Short:   "Create a Cloud VPS instance using image, flavor, and network IDs",
		Args:    cobra.ExactArgs(1),
		Example: "create ci-runner --project mwcli --image <image-id> --flavor <flavor-id> --network <network-id> --wait",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dnsLabel.MatchString(args[0]) {
				return fmt.Errorf("instance name must be a lowercase DNS label (up to 63 characters)")
			}
			if image == "" || flavor == "" || network == "" {
				return fmt.Errorf("--image, --flavor, and --network IDs are required; use cloudvps image/flavor/network list to discover them")
			}
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			opts := servers.CreateOpts{
				Name: args[0], ImageRef: image, FlavorRef: flavor,
				Networks:       []servers.Network{{UUID: network}},
				SecurityGroups: groups, AvailabilityZone: zone,
			}
			if userData != "" {
				data, err := os.ReadFile(userData)
				if err != nil {
					return fmt.Errorf("read --user-data: %w", err)
				}
				opts.UserData = data
			}
			p, err := provider(cmd)
			if err != nil {
				return err
			}
			client, err := openstack.NewComputeV2(p, gophercloud.EndpointOpts{})
			if err != nil {
				return err
			}
			server, err := servers.Create(cmd.Context(), client, keypairs.CreateOptsExt{
				CreateOptsBuilder: opts, KeyName: keyName,
			}, nil).Extract()
			if err != nil {
				return fmt.Errorf("create instance: %w", err)
			}
			// Creation may return an admin password; do not include it in output.
			server.AdminPass = ""
			out.Print(cmd, server)
			if wait {
				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				defer cancel()
				if err := waitForResource(ctx, "ACTIVE", func(ctx context.Context) (string, error) {
					current, err := servers.Get(ctx, client, server.ID).Extract()
					if err != nil {
						return "", err
					}
					return current.Status, nil
				}); err != nil {
					return fmt.Errorf("instance %s was created but did not become ACTIVE: %w", server.ID, err)
				}
			}
			return nil
		},
	}
	addProjectFlag(cmd)
	cmd.Flags().StringVar(&image, "image", "", "Image ID (required; see cloudvps image list)")
	cmd.Flags().StringVar(&flavor, "flavor", "", "Flavor ID (required; see cloudvps flavor list)")
	cmd.Flags().StringVar(&network, "network", "", "Network ID (required; see cloudvps network list)")
	cmd.Flags().StringVar(&keyName, "key-name", "", "Existing OpenStack SSH key pair name (optional)")
	cmd.Flags().StringSliceVar(&groups, "security-group", nil, "Security group names (repeatable or comma-separated; defaults to OpenStack default)")
	cmd.Flags().StringVar(&zone, "availability-zone", "", "Availability zone (optional)")
	cmd.Flags().StringVar(&userData, "user-data", "", "Path to a cloud-init user-data file (optional)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the instance to become ACTIVE (does not wait for cloud-init or SSH)")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Maximum time to wait for ACTIVE")
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType))
	return cmd
}
