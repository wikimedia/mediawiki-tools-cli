package gerrit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotgitreview"
)

func NewGerritDotGitReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dotgitreview",
		Short: "Interact with Gerrit .gitreview files",
	}
	cmd.AddCommand(NewGerritDotGitReviewProjectCmd())
	cmd.AddCommand(NewGerritDotGitReviewHostCmd())
	cmd.AddCommand(NewGerritDotGitReviewPortCmd())
	return cmd
}

func NewGerritDotGitReviewProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "project",
		Short: "Current Gerrit project from .gitreview file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(gitReviewOrExit().Project)
		},
	}
}

func NewGerritDotGitReviewHostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "host",
		Short: "Current Gerrit host from .gitreview file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(gitReviewOrExit().Host)
		},
	}
}

func NewGerritDotGitReviewPortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "port",
		Short: "Current Gerrit port from .gitreview file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(gitReviewOrExit().Port)
		},
	}
}

func gitReviewOrExit() *dotgitreview.GitReview {
	gitReview, err := dotgitreview.ForCWD()
	if err != nil {
		fmt.Println("Failed to get .gitreview file, are you in a Gerrit repository?")
		os.Exit(1)
	}

	return gitReview
}
