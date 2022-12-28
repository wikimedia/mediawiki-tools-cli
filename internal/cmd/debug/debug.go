package debug

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func debugCommandsAreHidden() bool {
	return cli.VersionDetails.Version != "latest"
}

func NewDebugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "debug",
		Short:  "mwcli debug commands (only in dev builds)",
		Hidden: debugCommandsAreHidden(),
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Debug"
	cmd.AddCommand(NewDebugEventsCmd())
	return cmd
}
