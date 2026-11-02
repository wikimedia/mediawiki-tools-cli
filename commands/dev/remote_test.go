package dev

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devremote"
)

func TestRemoteOpenDoesNotInitializeLocalDocker(t *testing.T) {
	// An invalid connection is rejected before invoking SSH. Parent setup must
	// not check Docker, create a local environment, or change its .env file.
	root := &cobra.Command{Use: "mw", SilenceErrors: true, SilenceUsage: true}
	root.AddGroup(&cobra.Group{ID: "dev", Title: "Development"})
	rootCalls := 0
	root.PersistentPreRun = func(*cobra.Command, []string) { rootCalls++ }
	root.AddCommand(NewCmd())
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	t.Setenv("PATH", t.TempDir()) // No Docker or SSH is installed on the client.
	root.SetArgs([]string{"dev", "remote", "open", "--ssh=-invalid", "--editor", "none"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "SSH hostname") {
		t.Fatalf("unexpected result: %v\n%s", err, output.String())
	}
	if rootCalls != 1 {
		t.Fatalf("root initialization called %d times, want 1", rootCalls)
	}
}

func TestExplicitRemoteContextCannotShadowLocalEnvironment(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	local := filepath.Join(cli.UserDirectoryPathForCmd("mwdd"), "existing-local")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "mw", SilenceErrors: true, SilenceUsage: true}
	root.AddGroup(&cobra.Group{ID: "dev", Title: "Development"})
	root.AddCommand(NewCmd())
	root.SetArgs([]string{"dev", "remote", "open", "existing-local", "--ssh", "vm", "--editor", "none"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "already belongs to a local environment") {
		t.Fatalf("local context was not protected: %v", err)
	}
	if _, err := devremote.LoadContext("existing-local"); !os.IsNotExist(err) {
		t.Fatal("created a shadowing remote registration")
	}
}
