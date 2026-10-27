package main

import (
	"strings"
	"testing"
)

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
