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

func TestDefaultDockerfilePath(t *testing.T) {
	dir := "/some/mwdd/dir"
	cases := []struct {
		service string
		want    string
	}{
		{"mediawiki", filepath.Join(dir, "Dockerfile.mediawiki")},
		{"mysql", filepath.Join(dir, "Dockerfile.mysql")},
		{"shellbox-media", filepath.Join(dir, "Dockerfile.shellbox-media")},
	}
	for _, tc := range cases {
		got := defaultDockerfilePath(dir, tc.service)
		if got != tc.want {
			t.Errorf("defaultDockerfilePath(%q, %q) = %q, want %q", dir, tc.service, got, tc.want)
		}
	}
}

func TestDefaultImageForService(t *testing.T) {
	// mediawiki is a known embedded service – verify we get a non-empty default image.
	got := defaultImageForService("mediawiki")
	if got == "" {
		t.Errorf("defaultImageForService(%q) returned empty string, want a base image", "mediawiki")
	}
	if !strings.Contains(got, "docker-registry.wikimedia.org") {
		t.Errorf("defaultImageForService(%q) = %q, expected a wikimedia registry image", "mediawiki", got)
	}

	// Unknown service should return empty string.
	if img := defaultImageForService("nonexistent-service"); img != "" {
		t.Errorf("defaultImageForService(%q) = %q, want empty string", "nonexistent-service", img)
	}
}

func TestCreateStarterDockerfileIfNotExists_Creates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile.mediawiki")

	if err := createStarterDockerfileIfNotExists(path, "mediawiki"); err != nil {
		t.Fatalf("createStarterDockerfileIfNotExists: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading created file: %v", err)
	}
	got := string(content)

	if !strings.Contains(got, "mediawiki") {
		t.Errorf("starter Dockerfile does not mention the service name; got:\n%s", got)
	}
	if !strings.Contains(got, "FROM ") {
		t.Errorf("starter Dockerfile has no FROM line; got:\n%s", got)
	}
	if !strings.Contains(got, "USER www-data") {
		t.Errorf("starter Dockerfile does not restore user with 'USER www-data'; got:\n%s", got)
	}
}

func TestCreateStarterDockerfileIfNotExists_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile.mediawiki")

	original := "FROM scratch\n# user content\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("pre-creating file: %v", err)
	}

	// Should not overwrite the existing file.
	if err := createStarterDockerfileIfNotExists(path, "mediawiki"); err != nil {
		t.Fatalf("createStarterDockerfileIfNotExists: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after no-op call: %v", err)
	}
	if string(content) != original {
		t.Errorf("file was overwritten; got:\n%s\nwant:\n%s", string(content), original)
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
