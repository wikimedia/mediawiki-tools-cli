package mediawiki

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/sirupsen/logrus"
)

// GerritChangeInfo holds the relevant details extracted from a Gerrit change.
type GerritChangeInfo struct {
	Number   int
	ChangeID string
	Project  string
	Subject  string
	Ref      string
	FetchURL string
	Message  string
	DependsOn []string
}

var dependsOnLineRE = regexp.MustCompile(`(?im)^\s*Depends-On:\s*([^\s#]+)\s*$`)

func isLikelyGerritChangeID(changeRef string) bool {
	if len(changeRef) != 41 {
		return false
	}
	if changeRef[0] != 'I' {
		return false
	}
	for i := 1; i < len(changeRef); i++ {
		c := changeRef[i]
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		if !isDigit && !isLowerHex {
			return false
		}
	}
	return true
}

func buildGerritQuery(changeRef, preferredBranch string) string {
	parts := []string{"change:" + changeRef}
	if preferredBranch != "" {
		parts = append(parts, "branch:"+preferredBranch)
	}
	return strings.Join(parts, " ")
}

func describeAmbiguousChanges(changes []gerrit.ChangeInfo) string {
	max := 5
	if len(changes) < max {
		max = len(changes)
	}
	parts := make([]string, 0, max)
	for i := 0; i < max; i++ {
		c := changes[i]
		parts = append(parts, fmt.Sprintf("%d (%s|%s|%s)", c.Number, c.Project, c.Branch, c.Status))
	}
	return strings.Join(parts, ", ")
}

func filterByBranch(changes []gerrit.ChangeInfo, branch string) []gerrit.ChangeInfo {
	if branch == "" {
		return changes
	}
	out := make([]gerrit.ChangeInfo, 0, len(changes))
	for _, c := range changes {
		if c.Branch == branch {
			out = append(out, c)
		}
	}
	return out
}

func filterOpenChanges(changes []gerrit.ChangeInfo) []gerrit.ChangeInfo {
	out := make([]gerrit.ChangeInfo, 0, len(changes))
	for _, c := range changes {
		if c.Status == "NEW" || c.Status == "DRAFT" {
			out = append(out, c)
		}
	}
	return out
}

func querySingleChange(ctx context.Context, client *gerrit.Client, changeRef, preferredBranch string) (*gerrit.ChangeInfo, error) {
	query := buildGerritQuery(changeRef, preferredBranch)
	changes, _, err := client.Changes.QueryChanges(ctx, &gerrit.QueryChangeOptions{
		QueryOptions: gerrit.QueryOptions{
			Query: []string{query},
			Limit: 25,
		},
		ChangeOptions: gerrit.ChangeOptions{
			AdditionalFields: []string{"CURRENT_REVISION", "CURRENT_COMMIT"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying change by Change-Id with %q: %w", query, err)
	}

	if changes == nil || len(*changes) == 0 {
		return nil, nil
	}

	results := *changes
	if len(results) == 1 {
		return &results[0], nil
	}

	open := filterOpenChanges(results)
	if len(open) == 1 {
		return &open[0], nil
	}

	if preferredBranch == "" {
		branchFiltered := filterByBranch(results, "master")
		if len(branchFiltered) == 1 {
			return &branchFiltered[0], nil
		}
	}

	return nil, fmt.Errorf("multiple changes found for %s with query %q: %s", changeRef, query, describeAmbiguousChanges(results))
}

// FetchGerritChangeInfo retrieves key details about a Gerrit change.
func FetchGerritChangeInfo(ctx context.Context, changeID, preferredBranch, preferredProject string) (*GerritChangeInfo, error) {
	client, err := gerrit.NewClient(ctx, "https://gerrit.wikimedia.org/r/", nil)
	if err != nil {
		return nil, fmt.Errorf("creating Gerrit client: %w", err)
	}

	change, _, err := client.Changes.GetChange(ctx, changeID, &gerrit.ChangeOptions{
		AdditionalFields: []string{"CURRENT_REVISION", "CURRENT_COMMIT"},
	})
	if err != nil {
		if !isLikelyGerritChangeID(changeID) {
			return nil, fmt.Errorf("fetching change %s: %w", changeID, err)
		}

		resolved, queryErr := querySingleChange(ctx, client, changeID, preferredBranch)
		if queryErr != nil {
			return nil, fmt.Errorf("fetching change %s: %w", changeID, queryErr)
		}
		if resolved == nil {
			queryDesc := buildGerritQuery(changeID, preferredBranch)
			return nil, fmt.Errorf("fetching change %s: not found via query %q", changeID, queryDesc)
		}
		change = resolved
	}

	if change.CurrentRevision == "" {
		return nil, fmt.Errorf("change %s has no current revision", changeID)
	}

	revision, ok := change.Revisions[change.CurrentRevision]
	if !ok {
		return nil, fmt.Errorf("change %s: revision %s not found in revisions map", changeID, change.CurrentRevision)
	}

	fetchURL := ""
	ref := revision.Ref
	message := revision.Commit.Message
	if message == "" {
		message = revision.MessageWithFooter
	}
	dependsOn := extractDependsOnChangeIDs(message)
	if fetchInfo, ok := revision.Fetch["anonymous http"]; ok {
		fetchURL = fetchInfo.URL
		if fetchInfo.Ref != "" {
			ref = fetchInfo.Ref
		}
	} else {
		fetchURL = "https://gerrit.wikimedia.org/r/" + change.Project
	}

	return &GerritChangeInfo{
		Number:   change.Number,
		ChangeID: change.ChangeID,
		Project:  change.Project,
		Subject:  change.Subject,
		Ref:      ref,
		FetchURL: fetchURL,
		Message:  message,
		DependsOn: dependsOn,
	}, nil
}

func extractDependsOnChangeIDs(message string) []string {
	matches := dependsOnLineRE.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		changeID := strings.TrimSpace(m[1])
		if changeID == "" {
			continue
		}
		if _, ok := seen[changeID]; ok {
			continue
		}
		seen[changeID] = struct{}{}
		result = append(result, changeID)
	}
	return result
}

// ProjectToLocalDir maps a Gerrit project name to the relative local directory
// within the MediaWiki installation.
func ProjectToLocalDir(project string) (string, error) {
	if project == "mediawiki/core" {
		return "", nil
	}
	if strings.HasPrefix(project, "mediawiki/extensions/") {
		ext := strings.TrimPrefix(project, "mediawiki/extensions/")
		return "extensions/" + ext, nil
	}
	if strings.HasPrefix(project, "mediawiki/skins/") {
		skin := strings.TrimPrefix(project, "mediawiki/skins/")
		return "skins/" + skin, nil
	}
	return "", fmt.Errorf("unsupported Gerrit project: %s (only mediawiki/core, mediawiki/extensions/*, and mediawiki/skins/* are supported)", project)
}

// ApplyPatchOpts holds options for applying Gerrit patches.
type ApplyPatchOpts struct {
	ChangeIDs        []string
	DryRun           bool
	Mode             string
	WithDependencies bool
	CloneMissing     bool
	StartBranch      string
}

func resolveGerritChangePlan(ctx context.Context, roots []string, includeDependencies bool, startBranch string) ([]*GerritChangeInfo, error) {
	type visitState int
	const (
		stateVisiting visitState = 1
		stateDone     visitState = 2
	)

	stateByChangeNumber := map[int]visitState{}
	ordered := make([]*GerritChangeInfo, 0, len(roots))
	cacheByQuery := map[string]*GerritChangeInfo{}

	var visit func(changeQuery, preferredProject string) error
	visit = func(changeQuery, preferredProject string) error {
		query := strings.TrimSpace(changeQuery)
		if query == "" {
			return nil
		}
		cacheKey := query + "|project=" + preferredProject + "|branch=" + startBranch

		info, ok := cacheByQuery[cacheKey]
		if !ok {
			fetched, err := FetchGerritChangeInfo(ctx, query, startBranch, preferredProject)
			if err != nil {
				return fmt.Errorf("fetching change %s: %w", query, err)
			}
			info = fetched
			cacheByQuery[cacheKey] = info
		}

		switch stateByChangeNumber[info.Number] {
		case stateDone:
			return nil
		case stateVisiting:
			logrus.Warnf("Detected cyclic Depends-On reference at change %s (%s); skipping recursive dependency edge", strconv.Itoa(info.Number), info.ChangeID)
			return nil
		}

		stateByChangeNumber[info.Number] = stateVisiting

		if includeDependencies {
			for _, dep := range info.DependsOn {
				if err := visit(dep, info.Project); err != nil {
					return fmt.Errorf("resolving Depends-On for change %s (%s): %w", strconv.Itoa(info.Number), info.ChangeID, err)
				}
			}
		}

		stateByChangeNumber[info.Number] = stateDone
		ordered = append(ordered, info)
		return nil
	}

	for _, root := range roots {
		if err := visit(root, ""); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}

// ApplyGerritPatches fetches and applies Gerrit changes onto the local MediaWiki checkout.
//
// Supported modes:
//   - checkout: fetch and checkout the patchset ref (detached HEAD)
//   - cherry-pick: fetch and cherry-pick FETCH_HEAD into current branch
func (m MediaWiki) ApplyGerritPatches(ctx context.Context, opts ApplyPatchOpts) error {
	exitIfNoGit()
	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = "checkout"
	}
	startBranch := strings.TrimSpace(opts.StartBranch)
	if startBranch == "" {
		startBranch = "master"
	}
	withDependencies := opts.WithDependencies
	cloneMissing := opts.CloneMissing

	if mode != "checkout" && mode != "cherry-pick" {
		return fmt.Errorf("unsupported --mode %q (must be checkout or cherry-pick)", mode)
	}

	plannedChanges, err := resolveGerritChangePlan(ctx, opts.ChangeIDs, withDependencies, startBranch)
	if err != nil {
		return err
	}

	for _, info := range plannedChanges {
		fmt.Printf("Processing Gerrit change %d (%s)...\n", info.Number, info.ChangeID)

		fmt.Printf("  Project: %s\n", info.Project)
		fmt.Printf("  Subject: %s\n", info.Subject)
		fmt.Printf("  Ref:     %s\n", info.Ref)

		relDir, err := ProjectToLocalDir(info.Project)
		if err != nil {
			return err
		}

		repoDir := m.Path(relDir)
		logrus.Debugf("Local directory for change: %s", repoDir)

		if opts.DryRun {
			fmt.Printf("  Would apply to: %s\n", repoDir)
			if _, err := os.Stat(repoDir + "/.git"); os.IsNotExist(err) {
				if cloneMissing {
					fmt.Printf("  Would clone missing repository for %s\n", info.Project)
				} else {
					fmt.Printf("  Repository missing at %s (would fail without --clone-missing)\n", repoDir)
				}
			}
			continue
		}

		if _, err := os.Stat(repoDir + "/.git"); os.IsNotExist(err) {
			if !cloneMissing {
				return fmt.Errorf("repository not found at %s for project %s (use --clone-missing to clone automatically)", repoDir, info.Project)
			}
			fmt.Printf("  Repository not found at %s, cloning...\n", repoDir)
			cloneURL := "https://gerrit.wikimedia.org/r/" + info.Project
			if err := runGitTTY("git", "clone", "--recurse-submodules", cloneURL, repoDir); err != nil {
				return fmt.Errorf("cloning %s: %w", info.Project, err)
			}
		}

		if err := stashIfDirty(repoDir); err != nil {
			return fmt.Errorf("stashing changes in %s: %w", repoDir, err)
		}

		fmt.Printf("  Fetching %s...\n", info.Ref)
		if err := runGitTTY("git", "-C", repoDir, "fetch", info.FetchURL, info.Ref); err != nil {
			return fmt.Errorf("fetching ref %s: %w", info.Ref, err)
		}

		switch mode {
		case "checkout":
			fmt.Printf("  Checking out fetched change...\n")
			if err := runGitTTY("git", "-C", repoDir, "checkout", "--detach", "FETCH_HEAD"); err != nil {
				return fmt.Errorf("checking out change %d (%s): %w", info.Number, info.Subject, err)
			}
		case "cherry-pick":
			fmt.Printf("  Cherry-picking...\n")
			if err := runGitTTY("git", "-C", repoDir, "cherry-pick", "FETCH_HEAD"); err != nil {
				fmt.Println("  Cherry-pick failed, aborting to restore repository state...")
				_ = runGitTTY("git", "-C", repoDir, "cherry-pick", "--abort")
				return fmt.Errorf("applying change %d (%s): cherry-pick conflict; no changes were made", info.Number, info.Subject)
			}
		}

		fmt.Printf("  Successfully processed change %d (%s) using %s mode\n", info.Number, info.Subject, mode)
	}

	return nil
}

// runGitTTY runs a git command with stdin/stdout/stderr connected to the terminal,
// returning any error rather than calling logrus.Fatal.
func runGitTTY(name string, args ...string) error {
	cmd := osexec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func stashIfDirty(repoDir string) error {
	cmd := osexec.Command("git", "-C", repoDir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		fmt.Printf("  Stashing uncommitted changes in %s...\n", repoDir)
		if err := runGitTTY("git", "-C", repoDir, "stash", "--include-untracked"); err != nil {
			return fmt.Errorf("stashing changes: %w", err)
		}
	}
	return nil
}
