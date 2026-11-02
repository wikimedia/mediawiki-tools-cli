package gerrit

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotgitreview"
)

func NewGerritDotGitReviewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dotgitreview", Short: "Interact with Gerrit .gitreview files"}
	cmd.AddCommand(NewGerritDotGitReviewProjectCmd(), NewGerritDotGitReviewHostCmd(), NewGerritDotGitReviewPortCmd())
	return cmd
}

func NewGerritDotGitReviewProjectCmd() *cobra.Command {
	return gitReviewFieldCmd("project", func(r *dotgitreview.GitReview) string { return r.Project })
}

func NewGerritDotGitReviewHostCmd() *cobra.Command {
	return gitReviewFieldCmd("host", func(r *dotgitreview.GitReview) string { return r.Host })
}

func NewGerritDotGitReviewPortCmd() *cobra.Command {
	return gitReviewFieldCmd("port", func(r *dotgitreview.GitReview) string { return r.Port })
}

func gitReviewFieldCmd(field string, value func(*dotgitreview.GitReview) string) *cobra.Command {
	return &cobra.Command{
		Use: field, Short: "Current Gerrit " + field + " from .gitreview file", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			review, err := dotgitreview.ForCWD()
			if err != nil {
				return fmt.Errorf("read .gitreview (run this command in a Gerrit checkout): %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), value(review))
			return err
		},
	}
}
