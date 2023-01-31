package debug

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
)

func NewDebugEventsCatCmd() *cobra.Command {
	return &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Use:     "cat",
		Aliases: []string{"list"},
		Short:   "List events pending submission",
		Run: func(cmd *cobra.Command, args []string) {
			for _, line := range eventlogging.RawEvents() {
				fmt.Println(line)
			}
		},
	}
}
