package cloudvps

import (
	"github.com/spf13/cobra"
)

func NewCloudVPSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cloudvps",
		Aliases: []string{"vps"},
		GroupID: "service",
		Short:   "Connect to and manage Wikimedia Cloud VPS instances",
		RunE:    nil,
	}

	cmd.AddCommand(NewComputeCmd())
	cmd.AddCommand(NewSSHCmd())
	cmd.AddCommand(NewAuthCmd())
	cmd.AddCommand(NewVolumeCmd())
	for _, resource := range []string{"image", "flavor", "network"} {
		cmd.AddCommand(newResourceListCmd(resource, authenticatedProvider))
	}

	return cmd
}
