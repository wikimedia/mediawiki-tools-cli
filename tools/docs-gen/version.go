package main

import (
	"os"
	"os/exec"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/releaseversion"
)

type documentationVersion struct {
	Label      string `json:"label"`
	Commit     string `json:"commit"`
	Unreleased bool   `json:"unreleased"`
	Modified   bool   `json:"modified"`
}

func gitOutput(args ...string) string {
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func currentDocumentationVersion() documentationVersion {
	version := documentationVersion{Commit: gitOutput("rev-parse", "--short", "HEAD")}
	ref := strings.TrimSpace(os.Getenv("DOCS_VERSION"))
	if ref == "" || ref == "latest" {
		ref = strings.TrimSpace(os.Getenv("CI_COMMIT_TAG"))
		if ref == "" {
			ref = gitOutput("describe", "--tags", "--exact-match", "HEAD")
		}
	}
	_, releaseError := releaseversion.Parse(ref)
	version.Modified = gitOutput("status", "--porcelain", "--untracked-files=normal") != ""
	version.Unreleased = releaseError != nil || version.Modified
	if ref == "" || ref == "latest" {
		ref = gitOutput("branch", "--show-current")
		if ref == "" {
			ref = os.Getenv("CI_COMMIT_REF_NAME")
		}
		if ref == "" {
			ref = "development"
		}
	}
	version.Label = ref
	if version.Unreleased {
		version.Label += " / unreleased"
	}
	if version.Commit != "" {
		version.Label += " · " + version.Commit
	}
	if version.Modified {
		version.Label += " · modified"
	}
	return version
}
