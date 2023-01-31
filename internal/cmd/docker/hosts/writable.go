package hosts

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsWritableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "writable",
		Short: "Checks if you can write to the needed hosts file",
		Run: func(cmd *cobra.Command, args []string) {
			if hosts.Writable() {
				fmt.Println("Hosts file writable")
			} else {
				fmt.Println("Hosts file not writable")
				os.Exit(1)
			}
		},
	}
}
