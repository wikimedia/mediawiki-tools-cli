package main

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type documentationVersion struct {
	Label      string `json:"label"`
	Commit     string `json:"commit"`
	Unreleased bool   `json:"unreleased"`
	Modified   bool   `json:"modified"`
	Notice     bool   `json:"notice"`
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
	notice := strings.TrimSpace(os.Getenv("DOCS_VERSION_NOTICE"))
	version.Notice = notice == "1" || strings.EqualFold(notice, "true")
	ref := strings.TrimSpace(os.Getenv("DOCS_VERSION"))
	if ref == "" || ref == "latest" {
		ref = strings.TrimSpace(os.Getenv("CI_COMMIT_TAG"))
		if ref == "" {
			ref = gitOutput("describe", "--tags", "--exact-match", "HEAD")
		}
	}
	release := regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-zA-Z0-9.-]+)?$`)
	// CLI initialization under CI_PROJECT_DIR can create these runtime files;
	// they are not documentation source modifications.
	version.Modified = gitOutput("status", "--porcelain", "--untracked-files=normal", "--", ".", ":(exclude)config.json", ":(exclude).events") != ""
	version.Unreleased = !release.MatchString(ref) || version.Modified
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

func versionedDocumentation(body string, version documentationVersion) string {
	if !version.Notice {
		return body
	}
	// Redirects must start with #REDIRECT; their target carries the version notice.
	if strings.HasPrefix(strings.TrimSpace(body), "#REDIRECT") {
		return body
	}
	message := "Documentation for this release."
	if version.Unreleased {
		message = "This documentation describes unreleased code; your installed release may differ."
	}
	return fmt.Sprintf("<!-- a-wikimedia-cli version notice -->\n{| class=\"wikitable\"\n| '''Documentation version:''' <nowiki>%s</nowiki><br />%s\n|}\n<!-- /a-wikimedia-cli version notice -->\n\n%s", html.EscapeString(version.Label), message, body)
}

func withoutVersionNotice(body string) string {
	const end = "<!-- /a-wikimedia-cli version notice -->"
	if strings.HasPrefix(body, "<!-- a-wikimedia-cli version notice -->") {
		if index := strings.Index(body, end); index >= 0 {
			return strings.TrimLeft(body[index+len(end):], "\n")
		}
	}
	return body
}
