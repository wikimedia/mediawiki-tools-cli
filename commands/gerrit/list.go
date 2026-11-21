package gerrit

import (
	"context"
	"fmt"
	"strings"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newListCmd() *cobra.Command {
	var mine, reviewing bool
	var limit int
	cmd := &cobra.Command{
		Use:               "list [query]",
		Short:             "List public changes or your Gerrit review queues",
		Long:              "List recently updated open public changes by default, or pass a Gerrit search query. --mine and --reviewing use the selected HTTP profile and show its open changes or review assignments.",
		Example:           "list --mine\nlist --reviewing\nlist 'project:mediawiki/core status:open'",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100")
			}
			query := "status:open"
			if len(args) > 0 {
				query = strings.TrimSpace(args[0])
				if query == "" {
					return fmt.Errorf("provide a nonempty Gerrit query")
				}
			}
			client, err := newGerritClient(cmd.Context())
			if err != nil {
				return err
			}
			if mine || reviewing {
				if err := authenticateReviewList(cmd, client); err != nil {
					return err
				}
				filter := "owner:self"
				if reviewing {
					filter = "reviewer:self"
				}
				query = "status:open " + filter
				if len(args) > 0 {
					query += " (" + strings.TrimSpace(args[0]) + ")"
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
			defer cancel()
			changes, err := readGerritChanges(ctx, client, query, limit)
			if err != nil {
				return fmt.Errorf("list Gerrit changes: %w", err)
			}
			printGerritList(cmd, changes)
			return nil
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "List open changes owned by the selected account")
	cmd.Flags().BoolVar(&reviewing, "reviewing", false, "List open changes assigned to the selected account for review")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum changes to show (1–100)")
	cmd.MarkFlagsMutuallyExclusive("mine", "reviewing")
	accesspolicy.Support(cmd)
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic, commandmeta.DataAccountMetadata, commandmeta.DataPrivateMetadata, commandmeta.DataSecrets},
	})
}

func authenticateReviewList(cmd *cobra.Command, client *gogerrit.Client) error {
	publicOnly, err := accesspolicy.CLI(cmd, "")
	if err != nil {
		return err
	}
	if publicOnly {
		return fmt.Errorf("--mine and --reviewing require authentication and cannot be used in public-only mode")
	}
	username, password, err := resolveGerritCredentials(cmd)
	if err != nil {
		return err
	}
	if username == "" || password == "" {
		return fmt.Errorf("no Gerrit credentials found; use mw gerrit auth login")
	}
	client.Authentication.SetBasicAuth(username, password)
	return nil
}

func printGerritList(cmd *cobra.Command, changes []gogerrit.ChangeInfo) {
	out := cmd.OutOrStdout()
	if len(changes) == 0 {
		fmt.Fprintln(out, "No matching Gerrit changes")
		return
	}
	table := output.NewTable([]any{"CHANGE", "STATUS", "PROJECT", "BRANCH", "SUBJECT"}, nil)
	for _, change := range changes {
		table.AddRowS(fmt.Sprint(change.Number), reviewStatus(change), reviewText(change.Project), reviewText(change.Branch), reviewText(change.Subject))
	}
	table.Print(out)
	if changes[len(changes)-1].MoreChanges {
		fmt.Fprintln(out, "More changes match. Increase --limit or narrow the query.")
	}
}
