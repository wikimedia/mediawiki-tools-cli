package k8s

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
)

/*NewServiceCmd a new command for a single service, such as mailhog*/
func NewServiceCmd(name string, long string, aliases []string) *cobra.Command {
	return &cobra.Command{
		Use:     name,
		Short:   fmt.Sprintf("%s service", name),
		Long:    cli.RenderMarkdown(long),
		Aliases: aliases,
		RunE:    nil,
	}
}

func NewServiceCreateCmd(name string, Verbosity int) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create the %s containers", name),
		Run: func(cmd *cobra.Command, args []string) {
			DefaultForUser().EnsureReady()
			DefaultForUser().ValuesFileExistsOrExit(name)
			fmt.Println("The values file existed")
			// deploy to k8s here
		},
	}
}
