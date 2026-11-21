package gerrit

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func newViewCmd() *cobra.Command {
	var patchset int
	cmd := &cobra.Command{
		Use:     "view [change]",
		Short:   "Show a Gerrit change's patchset, votes and unresolved threads",
		Long:    "Inspect a public change by number or URL, or use the uploaded patchset matching Git HEAD. Votes, submit requirements and unresolved threads reflect the current review state.",
		Example: "view\nview 1370181 --patchset 4",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, target, err := preparePublicChange(cmd.Context(), cmd, args, patchset)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
			defer cancel()
			change, selected, err := target.readChange(ctx, client, "DETAILED_LABELS", "DETAILED_ACCOUNTS", "SUBMIT_REQUIREMENTS")
			if err != nil {
				return err
			}
			comments, response, err := client.Changes.ListChangeComments(ctx, strconv.Itoa(target.change))
			if err != nil {
				if response != nil {
					response.Body.Close()
				}
				return fmt.Errorf("read Gerrit review comments: %w", err)
			}
			printGerritView(cmd.OutOrStdout(), change, selected, *comments)
			return nil
		},
	}
	return publicPatchsetCommand(cmd, &patchset)
}

func reviewStatus(change gogerrit.ChangeInfo) string {
	if change.Status == "NEW" && change.WorkInProgress {
		return "WIP"
	}
	return reviewText(change.Status)
}

func reviewText(value string) string {
	return strings.Join(strings.Fields(ansi.Strip(value)), " ")
}

func reviewAccount(account gogerrit.AccountInfo) string {
	if name := cmp.Or(account.DisplayName, account.Name, account.Username); name != "" {
		return reviewText(name)
	}
	if account.AccountID > 0 {
		return fmt.Sprintf("account %d", account.AccountID)
	}
	return "unknown account"
}

func printGerritView(out io.Writer, change *gogerrit.ChangeInfo, patchset int, comments map[string][]gogerrit.CommentInfo) {
	fmt.Fprintf(out, "%d  %s\n", change.Number, reviewText(change.Subject))
	fmt.Fprintf(out, "%s/%d\n", secrets.GerritEndpoint, change.Number)
	fmt.Fprintf(out, "Project: %s\nBranch: %s\nStatus: %s\nOwner: %s\n",
		reviewText(change.Project), reviewText(change.Branch), reviewStatus(*change), reviewAccount(change.Owner))
	fmt.Fprintf(out, "Patchset: %d", patchset)
	_, current, _ := gerritapi.Revision(change, 0)
	if latest := cmp.Or(change.CurrentRevisionNumber, current.Number); latest > 0 && latest != patchset {
		fmt.Fprintf(out, " (current: %d)", latest)
	}
	fmt.Fprintln(out)
	if commit, _, ok := gerritapi.Revision(change, patchset); ok {
		fmt.Fprintf(out, "Commit: %s\n", commit)
	}

	if len(change.Labels) != 0 {
		fmt.Fprintln(out, "\nVotes on the current patchset")
		for _, label := range slices.Sorted(maps.Keys(change.Labels)) {
			var votes []string
			for _, vote := range change.Labels[label].All {
				if vote.Value != 0 {
					votes = append(votes, fmt.Sprintf("%+d %s", vote.Value, reviewAccount(vote.AccountInfo)))
				}
			}
			if len(votes) == 0 {
				votes = append(votes, "no votes")
			}
			fmt.Fprintf(out, "  %s: %s\n", reviewText(label), strings.Join(votes, ", "))
		}
	}
	if len(change.SubmitRequirements) != 0 {
		fmt.Fprintln(out, "\nSubmit requirements")
		for _, requirement := range change.SubmitRequirements {
			fmt.Fprintf(out, "  %s: %s\n", reviewText(requirement.Name), reviewText(requirement.Status))
		}
	}

	threads := unresolvedReviewThreads(comments)
	fmt.Fprintf(out, "\nUnresolved threads: %d\n", len(threads))
	for _, thread := range threads {
		location := reviewText(thread.path)
		if thread.comment.Line > 0 {
			location += ":" + strconv.Itoa(thread.comment.Line)
		}
		fmt.Fprintf(out, "  %s (patchset %d, %s)\n    %s\n", location, thread.comment.PatchSet,
			reviewAccount(thread.comment.Author), reviewText(thread.comment.Message))
	}
}

type reviewThreadComment struct {
	path    string
	comment gogerrit.CommentInfo
}

func unresolvedReviewThreads(files map[string][]gogerrit.CommentInfo) []reviewThreadComment {
	byID := make(map[string]reviewThreadComment)
	replied := make(map[string]bool)
	for path, comments := range files {
		for _, comment := range comments {
			byID[comment.ID] = reviewThreadComment{path: path, comment: comment}
			replied[comment.InReplyTo] = true
		}
	}
	latest := make(map[string]reviewThreadComment)
	for id, entry := range byID {
		if replied[id] {
			continue
		}
		root := id
		// Gerrit links replies across patchsets. Bound the walk to handle a
		// malformed reply chain without looping forever.
		for range len(byID) {
			parent := byID[root].comment.InReplyTo
			if _, ok := byID[parent]; parent == "" || !ok {
				break
			}
			root = parent
		}
		if old, exists := latest[root]; !exists || commentOrder(old.comment, entry.comment) < 0 {
			latest[root] = entry
		}
	}
	var threads []reviewThreadComment
	for _, entry := range latest {
		if entry.comment.Unresolved != nil && *entry.comment.Unresolved {
			threads = append(threads, entry)
		}
	}
	slices.SortFunc(threads, func(a, b reviewThreadComment) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.comment.Line, b.comment.Line), commentOrder(a.comment, b.comment))
	})
	return threads
}

func commentOrder(a, b gogerrit.CommentInfo) int {
	if a.Updated != nil && b.Updated != nil {
		if order := a.Updated.Compare(b.Updated.Time); order != 0 {
			return order
		}
	}
	return cmp.Or(cmp.Compare(a.PatchSet, b.PatchSet), cmp.Compare(a.ID, b.ID))
}
