package devenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func TestDockerfileSetKeepsPathAcrossWorkingDirectories(t *testing.T) {
	t.Setenv("GITLAB_CI", "1")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	original := Context
	Context = "dockerfile-path-test"
	t.Cleanup(func() { Context = original })
	dir := t.TempDir()
	t.Chdir(dir)
	resolvedDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir = resolvedDir
	if err := os.WriteFile("Dockerfile", []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "wm"}
	service := &cobra.Command{Use: "memcached"}
	service.AddCommand(NewImageCmd("memcached"))
	root.AddCommand(service)
	root.SetArgs([]string{"memcached", "image", "dockerfile", "set", "Dockerfile"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	m := DefaultForUser()
	stored := m.Env().Get(dockerfileEnvKey("memcached"))
	if !filepath.IsAbs(stored) {
		t.Fatalf("stored a relative path: %q", stored)
	}
	t.Chdir(t.TempDir())
	m.EnsureReady()
	content, err := os.ReadFile(dockerfileComposeFilePath(m.Directory(), "memcached"))
	if err != nil {
		t.Fatal(err)
	}
	var override dockerfileComposeOverride
	if err := yaml.Unmarshal(content, &override); err != nil {
		t.Fatal(err)
	}
	if got := override.Services["memcached"].Build.Context; got != dir {
		t.Fatalf("build context changed after changing directory: got %q, want %q", got, dir)
	}
	if stored != filepath.Join(dir, "Dockerfile") {
		t.Fatalf("wrong stored path: %q", stored)
	}
}
