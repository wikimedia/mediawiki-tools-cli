package gerrit

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

//go:embed group_search.example
var groupSearchExample string

func NewGerritGroupSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "search [search string]...",
		Short:   "Search Gerrit groups",
		Args:    cobra.MinimumNArgs(1),
		Example: groupSearchExample,
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"ls-groups"}))
			out := cmdutil.AttachOutputBuffer(ssh)

			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}

			fmt.Println(stringsutil.FilterMultiline(out.String(), args))
		},
	}
}
