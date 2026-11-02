package dev

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devremote"
)

func TestRemoteWorkspaceDeletionRequiresConfirmation(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	previous := cli.Opts.NoInteraction
	cli.Opts.NoInteraction = true
	defer func() { cli.Opts.NoInteraction = previous }()
	c := devremote.Context{Options: devremote.Options{Context: "calm-otter", SSHArgs: []string{"offline-vm"}}, Workspace: "/home/test/wm/calm-otter", Metadata: "/home/test/wm/calm-otter/.wm", Source: "/home/test/wm/calm-otter/mediawiki"}
	if err := devremote.SaveContext(c); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "mw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(newRemoteRemoveCmd())
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"remove", "calm-otter", "--delete-workspace"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("missing confirmation: %v\n%s", err, &output)
	}
	if _, err := devremote.LoadContext(c.Context); err != nil {
		t.Fatal("removed the context without approval")
	}
	if !strings.Contains(output.String(), "database data and uncommitted") {
		t.Fatal("deletion preview did not explain what is lost")
	}
}

func TestRemoteCacheDryRunDoesNotDelete(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cache fixture uses XDG_CACHE_HOME")
	}
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	rootPath, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(rootPath, "a-wikimedia-cli", "remote-binaries", "v0.1.0", "linux-amd64", "wm")
	if err := os.MkdirAll(filepath.Dir(cached), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("cached"), 0o700); err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "mw", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(newRemoteCacheCmd())
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"cache", "cleanup", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Preview only") {
		t.Fatalf("missing preview: %s", &output)
	}
	if _, err := os.Stat(cached); err != nil {
		t.Fatal("dry run removed the cache")
	}
}
