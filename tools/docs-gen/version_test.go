package main

import "testing"

func TestDocumentationReleaseVersions(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, value := range []string{"v0.34.0", "v2026.11.15.0", "v2026.11.15.1"} {
		t.Setenv("DOCS_VERSION", value)
		version := currentDocumentationVersion()
		if version.Unreleased || version.Label != value {
			t.Fatalf("release %q marked as development: %+v", value, version)
		}
	}
}
