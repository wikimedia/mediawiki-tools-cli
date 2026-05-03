package mediawiki

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/exec"
)

// GerritChangeInfo holds the relevant details extracted from a Gerrit change.
type GerritChangeInfo struct {
	Number   int
	Project  string
	Subject  string
	Ref      string
	FetchURL string
}

// FetchGerritChangeInfo retrieves key details about a Gerrit change.
func FetchGerritChangeInfo(ctx context.Context, changeID string) (*GerritChangeInfo, error) {
	client, err := gerrit.NewClient(ctx, "https://gerrit.wikimedia.org/r/", nil)
	if err != nil {
		return nil, fmt.Errorf("creating Gerrit client: %w", err)
	}

	change, _, err := client.Changes.GetChange(ctx, changeID, &gerrit.ChangeOptions{
		AdditionalFields: []string{"CURRENT_REVISION"},
	})
	if err != nil {
		return nil, fmt.Errorf("fetching change %s: %w", changeID, err)
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
		Project:  change.Project,
		Subject:  change.Subject,
		Ref:      ref,
		FetchURL: fetchURL,
	}, nil
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
	ChangeIDs []string
	DryRun    bool
}

// ApplyGerritPatches fetches and cherry-picks Gerrit changes onto the local MediaWiki checkout.
func (m MediaWiki) ApplyGerritPatches(ctx context.Context, opts ApplyPatchOpts) error {
	exitIfNoGit()

	for _, changeID := range opts.ChangeIDs {
		fmt.Printf("Processing Gerrit change %s...\n", changeID)

		info, err := FetchGerritChangeInfo(ctx, changeID)
		if err != nil {
			return fmt.Errorf("change %s: %w", changeID, err)
		}

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
			continue
		}

		if _, err := os.Stat(repoDir + "/.git"); os.IsNotExist(err) {
			fmt.Printf("  Repository not found at %s, cloning...\n", repoDir)
			cloneURL := "https://gerrit.wikimedia.org/r/" + info.Project
			exec.RunTTYCommand(exec.Command("git", "clone", "--recurse-submodules", cloneURL, repoDir))
		}

		if err := stashIfDirty(repoDir); err != nil {
			return fmt.Errorf("stashing changes in %s: %w", repoDir, err)
		}

		fmt.Printf("  Fetching %s...\n", info.Ref)
		exec.RunTTYCommand(exec.Command("git", "-C", repoDir, "fetch", info.FetchURL, info.Ref))

		fmt.Printf("  Cherry-picking...\n")
		exec.RunTTYCommand(exec.Command("git", "-C", repoDir, "cherry-pick", "FETCH_HEAD"))

		fmt.Printf("  Successfully applied change %s (%s)\n", changeID, info.Subject)
	}

	return nil
}

func stashIfDirty(repoDir string) error {
	cmd := osexec.Command("git", "-C", repoDir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		fmt.Printf("  Stashing uncommitted changes in %s...\n", repoDir)
		exec.RunTTYCommand(exec.Command("git", "-C", repoDir, "stash", "--include-untracked"))
	}
	return nil
}
