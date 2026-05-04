package mwdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerfileEnvKey(t *testing.T) {
	cases := []struct {
		service string
		want    string
	}{
		{"mediawiki", "MEDIAWIKI_DOCKERFILE"},
		{"mysql", "MYSQL_DOCKERFILE"},
		{"shellbox-media", "SHELLBOX-MEDIA_DOCKERFILE"},
		{"mediawiki-web", "MEDIAWIKI-WEB_DOCKERFILE"},
	}
	for _, tc := range cases {
		got := dockerfileEnvKey(tc.service)
		if got != tc.want {
			t.Errorf("dockerfileEnvKey(%q) = %q, want %q", tc.service, got, tc.want)
		}
	}
}

func TestDockerfileComposeFilePath(t *testing.T) {
	dir := "/some/mwdd/dir"
	cases := []struct {
		service string
		want    string
	}{
		{"mediawiki", filepath.Join(dir, "custom-dockerfile-mediawiki.yml")},
		{"mysql", filepath.Join(dir, "custom-dockerfile-mysql.yml")},
		{"shellbox-media", filepath.Join(dir, "custom-dockerfile-shellbox-media.yml")},
	}
	for _, tc := range cases {
		got := dockerfileComposeFilePath(dir, tc.service)
		if got != tc.want {
			t.Errorf("dockerfileComposeFilePath(%q, %q) = %q, want %q", dir, tc.service, got, tc.want)
		}
	}
}

func TestWriteDockerfileComposeFile(t *testing.T) {
	dir := t.TempDir()

	// Create a fake Dockerfile to reference.
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatalf("creating temp Dockerfile: %v", err)
	}

	if err := writeDockerfileComposeFile(dir, "mediawiki", dockerfilePath); err != nil {
		t.Fatalf("writeDockerfileComposeFile: %v", err)
	}

	composePath := dockerfileComposeFilePath(dir, "mediawiki")
	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("reading generated compose file: %v", err)
	}

	got := string(content)

	// Verify the generated file contains the expected keys.
	checks := []string{
		"services:",
		"mediawiki:",
		"build:",
		"context: " + dir,
		"dockerfile: Dockerfile",
		"image: mwcli-mediawiki-custom:local",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("generated compose file does not contain %q\nfull content:\n%s", want, got)
		}
	}
}

func TestRemoveDockerfileComposeFile_Exists(t *testing.T) {
	dir := t.TempDir()
	path := dockerfileComposeFilePath(dir, "mediawiki")

	// Create the file first.
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("creating file: %v", err)
	}

	if err := removeDockerfileComposeFile(dir, "mediawiki"); err != nil {
		t.Fatalf("removeDockerfileComposeFile: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be removed, but it still exists")
	}
}

func TestRemoveDockerfileComposeFile_NotExists(t *testing.T) {
	dir := t.TempDir()
	// File doesn't exist – should be a no-op without error.
	if err := removeDockerfileComposeFile(dir, "mediawiki"); err != nil {
		t.Fatalf("removeDockerfileComposeFile on non-existent file: %v", err)
	}
}
