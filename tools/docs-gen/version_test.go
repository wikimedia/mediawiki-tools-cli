package main

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationSummaryOverridesKeepHistoryRecognition(t *testing.T) {
	t.Setenv("DOCS_VERSION", "v0.34.0")
	t.Setenv("DOCS_SUMMARY", "")
	summary, err := documentationEditSummary("main / unreleased · abc123 · modified", false)
	if err != nil || summary != publicationSummary+" (v0.34.0)" {
		t.Fatalf("release summary polluted by checkout metadata: %q, %v", summary, err)
	}
	t.Setenv("DOCS_VERSION", "")
	t.Setenv("CI_COMMIT_TAG", "v0.34.0")
	summary, err = documentationEditSummary("v0.34.0 · abc123 · modified", false)
	if err != nil || summary != publicationSummary+" (v0.34.0)" {
		t.Fatalf("tag publication summary polluted by checkout metadata: %q, %v", summary, err)
	}
	t.Setenv("DOCS_SUMMARY", "Refresh documentation for v0.34.0")
	summary, err = documentationEditSummary("main / unreleased", false)
	if err != nil || summary != "Refresh documentation for v0.34.0"+publicationSummarySuffix {
		t.Fatalf("custom summary not honored: %q, %v", summary, err)
	}
	if !automaticPublication(configuration{}, "Cli/ref/Test", snapshot{Comment: summary, Text: marked("generated")}) || automaticPublication(configuration{}, "Cli/ref/Test", snapshot{Comment: summary, Text: "manual correction"}) {
		t.Fatal("custom summary lost fingerprint checks or history discovery")
	}
	for _, invalid := range []string{"first\nsecond", strings.Repeat("x", 500)} {
		t.Setenv("DOCS_SUMMARY", invalid)
		if _, err := documentationEditSummary("v0.34.0", false); err == nil {
			t.Fatal("invalid summary accepted")
		}
	}
}

func TestRuntimeConfigurationDoesNotMarkDocumentationModified(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "fixture"}} {
		if out, err := osexec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, out)
		}
	}
	if err := os.WriteFile("config.json", []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if currentDocumentationVersion().Modified {
		t.Fatal("runtime config incorrectly marks source modified")
	}
	if err := os.WriteFile(filepath.Join("source.md"), []byte("new source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !currentDocumentationVersion().Modified {
		t.Fatal("untracked source modifications were ignored")
	}
}

func TestVersionNoticeIsOptIn(t *testing.T) {
	const body = "Documentation to publish before release.\n"
	for _, setting := range []string{"", "0", "false"} {
		t.Setenv("DOCS_VERSION_NOTICE", setting)
		version := currentDocumentationVersion()
		if got := versionedDocumentation(body, version); got != body {
			t.Fatalf("setting %q unexpectedly added notice: %s", setting, got)
		}
	}
	for _, setting := range []string{"1", "true"} {
		t.Setenv("DOCS_VERSION_NOTICE", setting)
		version := currentDocumentationVersion()
		version.Label = "feature / unreleased · abc123"
		version.Unreleased = true
		got := versionedDocumentation(body, version)
		if !strings.Contains(got, version.Label) || !strings.Contains(got, "This documentation describes unreleased code") {
			t.Fatalf("setting %q lost version notice: %s", setting, got)
		}
		if withoutVersionNotice(got) != body {
			t.Fatal("version notice could not be removed without changing content")
		}
		const redirect = "#REDIRECT [[Cli/ref/mw]]\n"
		if got := versionedDocumentation(redirect, version); got != redirect {
			t.Fatalf("notice broke redirect: %s", got)
		}
	}
}
