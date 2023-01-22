package gerrit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dotgitreview"
)

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
