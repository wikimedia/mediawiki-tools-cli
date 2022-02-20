package gerrit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dotgitreview"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func NewGerritProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Interact with Gerrit projects",
	}
	cmd.AddCommand(NewGerritProjectListCmd())
	cmd.AddCommand(NewGerritProjectSearchCmd())
	cmd.AddCommand(NewGerritProjectCurrentCmd())
	return cmd
}

func NewGerritProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Gerrit projects",
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-projects"}))
			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
		},
	}
}

func NewGerritProjectSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "search",
		Short:   "Search Gerrit projects",
		Example: "search mediawiki/extensions\nsearch Wikibase Lexeme",
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

func NewGerritProjectCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Detect current Gerrit project",
		Run: func(cmd *cobra.Command, args []string) {
			gitReview, err := dotgitreview.ForCWD()
			if err != nil {
				fmt.Println("Failed to get .gitreview file, are you in a Gerrit repository?")
				os.Exit(1)
			}

			fmt.Println(gitReview.Project)
		},
	}
}
