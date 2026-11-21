package gerrit

import (
	"context"
	"fmt"
	"net/url"
	osexec "os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
)

const gerritRequestTimeout = 10 * time.Second

var gerritChangePath = regexp.MustCompile(`^/r/(?:c/(?:[^/]+/)+\+/)?([0-9]+)(?:/([0-9]+))?/?$`)

func parseGerritChange(selector string, patchset int) (change, selectedPatchset int, err error) {
	selectedPatchset = patchset
	if number, err := strconv.Atoi(selector); err == nil && number > 0 {
		change = number
		return change, selectedPatchset, nil
	}
	address, err := url.Parse(selector)
	if err != nil || address.Scheme != "https" || address.Host != "gerrit.wikimedia.org" || address.User != nil {
		return change, selectedPatchset, fmt.Errorf("expected a Wikimedia Gerrit change number or URL, got %q", selector)
	}
	match := gerritChangePath.FindStringSubmatch(address.Path)
	if match == nil {
		return change, selectedPatchset, fmt.Errorf("Gerrit URL does not identify a change: %q", selector)
	}
	change, err = strconv.Atoi(match[1])
	if err != nil || change < 1 {
		return change, selectedPatchset, fmt.Errorf("invalid Gerrit change number in %q", selector)
	}
	if match[2] != "" {
		number, err := strconv.Atoi(match[2])
		if err != nil || number < 1 {
			return change, selectedPatchset, fmt.Errorf("invalid patchset number in %q", selector)
		}
		if patchset != 0 && patchset != number {
			return change, selectedPatchset, fmt.Errorf("--patchset %d conflicts with patchset %d in the URL", patchset, number)
		}
		selectedPatchset = number
	}
	return change, selectedPatchset, nil
}

type changeTarget struct {
	change   int
	patchset int
	commit   string
}

func selectChangeTarget(ctx context.Context, args []string, patchset int) (changeTarget, error) {
	target := changeTarget{patchset: patchset}
	var err error
	if len(args) == 0 {
		head, err := osexec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD").Output()
		if err != nil {
			return target, fmt.Errorf("cannot read Git HEAD. Pass a Gerrit change number or URL")
		}
		target.commit = strings.TrimSpace(string(head))
		return target, nil
	}
	target.change, target.patchset, err = parseGerritChange(args[0], patchset)
	return target, err
}

func (target *changeTarget) resolveChange(ctx context.Context, client *gogerrit.Client) error {
	if target.change != 0 {
		return nil
	}
	changes, err := readGerritChanges(ctx, client, "commit:"+target.commit, 2)
	if err != nil {
		return fmt.Errorf("look up Git HEAD in Gerrit: %w", err)
	}
	if len(changes) != 1 || changes[0].MoreChanges {
		return fmt.Errorf("Git HEAD did not match exactly one public Gerrit change. Pass a change number or URL")
	}
	target.change = changes[0].Number
	return nil
}

func (target changeTarget) readChange(ctx context.Context, client *gogerrit.Client, fields ...string) (*gogerrit.ChangeInfo, int, error) {
	field := "CURRENT_REVISION"
	if target.patchset != 0 || target.commit != "" || slices.Contains(fields, "ALL_REVISIONS") {
		field = "ALL_REVISIONS"
	}
	if !slices.Contains(fields, field) {
		fields = append(fields, field)
	}
	change, response, err := client.Changes.GetChangeDetail(ctx, strconv.Itoa(target.change), &gogerrit.ChangeOptions{
		AdditionalFields: fields,
	})
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		return nil, 0, fmt.Errorf("read Gerrit change %d: %w", target.change, err)
	}
	patchset := target.patchset
	if patchset == 0 && target.commit != "" {
		patchset = change.Revisions[target.commit].Number
		if patchset == 0 {
			return nil, 0, fmt.Errorf("Git HEAD is not a patchset of Gerrit change %d. Pass a change number or URL", target.change)
		}
	}
	if patchset == 0 {
		patchset = change.CurrentRevisionNumber
		if patchset == 0 {
			_, revision, _ := gerritapi.Revision(change, 0)
			patchset = revision.Number
		}
	}
	if patchset > 0 && patchset == change.CurrentRevisionNumber {
		return change, patchset, nil
	}
	if _, _, ok := gerritapi.Revision(change, patchset); patchset > 0 && ok {
		return change, patchset, nil
	}
	return nil, 0, fmt.Errorf("patchset %d is unavailable for Gerrit change %d", patchset, target.change)
}

func preparePublicChange(ctx context.Context, cmd *cobra.Command, args []string, patchset int) (*gogerrit.Client, changeTarget, error) {
	ctx, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
	defer cancel()
	if cmd.Flags().Changed("patchset") && patchset < 1 {
		return nil, changeTarget{}, fmt.Errorf("--patchset must be positive")
	}
	target, err := selectChangeTarget(ctx, args, patchset)
	if err != nil {
		return nil, changeTarget{}, err
	}
	client, err := newGerritClient(ctx)
	if err != nil {
		return nil, changeTarget{}, err
	}
	if err := target.resolveChange(ctx, client); err != nil {
		return nil, changeTarget{}, err
	}
	return client, target, nil
}

func publicPatchsetCommand(cmd *cobra.Command, patchset *int) *cobra.Command {
	accesspolicy.Support(cmd)
	commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic},
	})
	cmd.Flags().IntVar(patchset, "patchset", 0, "Inspect this patchset")
	cmd.ValidArgsFunction = cobra.NoFileCompletions
	return cmd
}

func readGerritChanges(ctx context.Context, client *gogerrit.Client, query string, limit int, fields ...string) ([]gogerrit.ChangeInfo, error) {
	changes, response, err := client.Changes.QueryChanges(ctx, &gogerrit.QueryChangeOptions{
		QueryOptions:  gogerrit.QueryOptions{Query: []string{query}, Limit: limit},
		ChangeOptions: gogerrit.ChangeOptions{AdditionalFields: fields},
	})
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		return nil, err
	}
	return *changes, nil
}
