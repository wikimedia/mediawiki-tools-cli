package hosts

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show your system hosts file",
		RunE: func(cmd *cobra.Command, args []string) error {
			content := hosts.FileContent()
			trimmedContent := strings.TrimSuffix(content, "\n")
			fmt.Fprintln(cmd.OutOrStdout(), trimmedContent)
			return nil
		},
	}
}
