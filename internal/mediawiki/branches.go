package mediawiki

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type CheckoutBranch struct {
	Name    string
	Branch  string
	Warning string
}

// CheckoutBranches reports branch names without fetching or modifying repositories.
// Only known release/development branch mismatches are classified as warnings:
// arbitrary feature branches and detached HEADs cannot establish compatibility.
func (m MediaWiki) CheckoutBranches() ([]CheckoutBranch, error) {
	core, err := gitCheckoutBranch(m.Directory())
	if err != nil {
		return nil, err
	}
	coreCheckout := CheckoutBranch{Name: "core", Branch: core}
	if strings.HasPrefix(core, "detached@") {
		coreCheckout.Warning = fmt.Sprintf("core is detached at %s", strings.TrimPrefix(core, "detached@"))
	}
	result := []CheckoutBranch{coreCheckout}
	for _, parent := range []string{"skins", "extensions"} {
		entries, err := os.ReadDir(m.Path(parent))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			directory := m.Path(filepath.Join(parent, entry.Name()))
			// Git would otherwise report the enclosing core checkout for plain folders.
			if _, err := os.Stat(filepath.Join(directory, ".git")); err != nil {
				continue
			}
			branch, err := gitCheckoutBranch(directory)
			name := parent + "/" + entry.Name()
			if err != nil {
				result = append(result, CheckoutBranch{Name: name, Branch: "unknown", Warning: err.Error()})
				continue
			}
			warning := ""
			if strings.HasPrefix(branch, "detached@") {
				warning = fmt.Sprintf("%s is detached at %s", name, strings.TrimPrefix(branch, "detached@"))
			} else if branchesLikelyIncompatible(core, branch) {
				warning = fmt.Sprintf("%s uses %s while core uses %s; check release compatibility", name, branch, core)
			}
			result = append(result, CheckoutBranch{Name: name, Branch: branch, Warning: warning})
		}
	}
	return result, nil
}

func gitCheckoutBranch(directory string) (string, error) {
	output, err := osexec.Command("git", "-C", directory, "symbolic-ref", "--quiet", "--short", "HEAD").Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}
	output, err = osexec.Command("git", "-C", directory, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("cannot read Git branch for %s: %w", directory, err)
	}
	return "detached@" + strings.TrimSpace(string(output)), nil
}

var mediawikiReleaseBranch = regexp.MustCompile(`^REL[0-9]+_[0-9]+$`)

func branchesLikelyIncompatible(core, other string) bool {
	family := func(branch string) string {
		if branch == "master" || branch == "main" {
			return "development"
		}
		if mediawikiReleaseBranch.MatchString(branch) {
			return branch
		}
		return ""
	}
	a, b := family(core), family(other)
	return a != "" && b != "" && a != b
}
