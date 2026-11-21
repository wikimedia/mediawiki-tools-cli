package gerrit

import (
	"fmt"

	"github.com/spf13/cobra"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
)

func newFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch <change>",
		Short:   "Fetch a Gerrit patchset into FETCH_HEAD",
		Long:    "Fetch a public Wikimedia Gerrit change into FETCH_HEAD in a repository with a matching remote. Select an older patchset with --patchset or a patchset URL. The branch, index and working files stay in place, including uncommitted edits.",
		Example: "fetch 1370181\nfetch 1370181 --patchset 4",
	}
	return patchsetTransferCommand(cmd, fetchChange)
}

func fetchChange(cmd *cobra.Command, number, patchset int) error {
	ctx := cmd.Context()
	if _, err := gitOutput(ctx, "", "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("run inside the change's Git repository")
	}
	fetched, err := fetchGerritPatchset(cmd, number, patchset, "--write-fetch-head")
	if err != nil {
		return err
	}
	head, err := gitOutput(ctx, "", "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil || head != fetched.commit {
		return fmt.Errorf("FETCH_HEAD does not match Gerrit change %d, patchset %d; retry the fetch", number, fetched.patchset)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Fetched change %d, patchset %d (%s) into FETCH_HEAD\n", number, fetched.patchset, fetched.project)
	return err
}

type fetchedPatchset struct {
	commit, project string
	patchset        int
}

func fetchGerritPatchset(cmd *cobra.Command, number, patchset int, fetchHeadFlag string) (fetchedPatchset, error) {
	ctx := cmd.Context()
	change, err := checkoutChangeInfo(ctx, number, patchset)
	if err != nil {
		return fetchedPatchset{}, err
	}
	commit, revision, ok := gerritapi.Revision(change, patchset)
	if !ok {
		if patchset == 0 {
			return fetchedPatchset{}, fmt.Errorf("current patchset is unavailable for Gerrit change %d", number)
		}
		return fetchedPatchset{}, fmt.Errorf("patchset %d is unavailable for Gerrit change %d", patchset, number)
	}
	if err := verifyCheckoutRepository(ctx, change.Project, cmd.Root().Name(), number); err != nil {
		return fetchedPatchset{}, err
	}
	address, ref, err := verifiedRevisionFetch(change.Project, number, commit, revision)
	if err != nil {
		return fetchedPatchset{}, err
	}
	if err := runGerritGit(cmd, "", "fetch", "--no-tags", "--no-recurse-submodules", "--no-prune", "--no-auto-maintenance",
		"--refmap=", fetchHeadFlag, "--", address, ref); err != nil {
		return fetchedPatchset{}, fmt.Errorf("fetch Gerrit change %d, patchset %d: %w", number, revision.Number, err)
	}
	return fetchedPatchset{commit: commit, project: change.Project, patchset: revision.Number}, nil
}
