package gerrit

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

//go:embed project_search.example
var projectSearchExample string

func NewGerritProjectSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "search",
		Short:   "Search Gerrit projects",
		Example: projectSearchExample,
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"ls-projects"}))
			out := cmdutil.AttachOutputBuffer(ssh)

			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}

			fmt.Println(stringsutil.FilterMultiline(out.String(), args))
		},
	}
}
