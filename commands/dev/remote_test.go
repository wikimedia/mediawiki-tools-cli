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

func TestRemoteMachineLifecycleProtectsAttachedCloudVPS(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	target := &devremote.ProviderTarget{Kind: "cloudvps", Project: "mwcli", ID: "existing-vm"}
	c := devremote.Context{Options: devremote.Options{Context: "attached"}, Provider: target}
	if err := devremote.SaveContext(c); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"stop", "destroy"} {
		t.Run(action, func(t *testing.T) {
			root := NewRemoteCmd()
			args := []string{action, "attached"}
			if action == "destroy" {
				args = append(args, "--yes")
			}
			root.SetArgs(args)
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "only machines created by mw") {
				t.Fatalf("attached machine lifecycle was not rejected: %v", err)
			}
		})
	}
}

func TestRemoteProviderContextSavesMachineBeforeSSHIsReady(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	c := devremote.Context{
		Options:  devremote.Options{Context: "pending", Recipe: "none"},
		Provider: &devremote.ProviderTarget{Kind: "codespaces", ID: "new-machine", Owned: true},
	}
	if err := devremote.SaveContext(c); err != nil {
		t.Fatal(err)
	}
	loaded, err := devremote.LoadContext("pending")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider == nil || *loaded.Provider != *c.Provider {
		t.Fatalf("saved target changed: %+v", loaded.Provider)
	}
	c.SSHArgs = []string{"-F", "provider-ssh.conf", "cs.new-machine.main"}
	if err := devremote.SaveContext(c); err != nil {
		t.Fatalf("could not update the SSH connection for the same machine: %v", err)
	}
	c.Provider.ID = "another-machine"
	if err := devremote.SaveContext(c); err == nil {
		t.Fatal("context moved to another machine")
	}
}
