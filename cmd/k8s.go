package cmd

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/k8s"
)

func NewK8sCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "k8s",
		Short: "Interaction with a kubernetes cluster",
		RunE:  nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Root().PersistentPreRun(cmd, args)
			k8s.DefaultForUser().EnsureReady()
		},
	}
}

func k8sAttachtoCmd() *cobra.Command {
	k8sCmd := NewK8sCmd()
	mediawiki := k8s.NewServiceCmd("mediawiki", "", []string{})
	mediawiki.AddCommand(k8s.NewServiceCreateCmd("mediawiki", globalOpts.Verbosity))
	k8sCmd.AddCommand(mediawiki)

	return k8sCmd
}
