package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/util/dotgitreview"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

func NewGerritProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "project",
		Short: "Interact with Gerrit projects",
	}
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
		Use:   "search",
		Short: "Search Gerrit projects",
		Example: `  search mediawiki/extensions
	  search Wikibase Lexeme`,
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
