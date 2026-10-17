package cloudvps

import (
	"github.com/spf13/cobra"
)

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
	cmd.AddCommand(NewVolumeCmd())
	for _, resource := range []string{"image", "flavor", "network"} {
		cmd.AddCommand(newResourceListCmd(resource, authenticatedProvider))
	}

	return cmd
}
