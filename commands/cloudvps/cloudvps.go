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

	var profile string
	cmd.PersistentFlags().StringVar(&profile, "profile", "", "Saved Cloud VPS credential profile (login defaults to the project name)")
	_ = cmd.RegisterFlagCompletionFunc("profile", cloudProfiles.Complete)

	cmd.AddCommand(NewComputeCmd())
	cmd.AddCommand(NewSSHCmd())
	cmd.AddCommand(NewAuthCmd(&profile))
	cmd.AddCommand(NewVolumeCmd())
	for _, resource := range []string{"image", "flavor", "network"} {
		cmd.AddCommand(newResourceListCmd(resource, authenticatedProvider))
	}

	registerProjectCompletions(cmd)
	return cmd
}
