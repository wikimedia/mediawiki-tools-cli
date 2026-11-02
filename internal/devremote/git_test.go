package devremote

import (
	"bytes"
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitRejectsExecutableTransportsAndCredentialURLs(t *testing.T) {
	for _, raw := range []string{"ext::sh evil", "https://user:secret@example.org/repo.git", "https://example.org/repo?token=secret", "ssh://user:secret@example.org/repo", "host;evil:repo", "-upload-pack=evil"} {
		if ValidateGitRemote(raw) == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"https://gitlab.wikimedia.org/repos/project.git", "ssh://user@gerrit.wikimedia.org:29418/project", "user@gerrit.wikimedia.org:project", "/tmp/bare.git"} {
		if err := ValidateGitRemote(raw); err != nil {
			t.Fatalf("rejected %q: %v", raw, err)
		}
	}
}

func TestGitBridgeTransfersObjectsAndKeepsAuthenticationLocal(t *testing.T) {
	registryForTest(t)
	fakeSSH(t)
	if _, err := osexec.LookPath("git"); err != nil {
		t.Skip("Git unavailable")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := osexec.Command("git", append([]string{"-c", "user.name=Remote Test", "-c", "user.email=test@example.invalid"}, args...)...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	upstream := filepath.Join(root, "upstream.git")
	git("init", "--bare", "-b", "main", upstream)
	seed := filepath.Join(root, "seed")
	git("init", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "file.txt"), []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("-C", seed, "add", ".")
	git("-C", seed, "commit", "-m", "initial")
	git("-C", seed, "push", upstream, "main")
	workspace := filepath.Join(root, "vm")
	c := Context{Options: Options{Context: "calm-otter", SSHArgs: []string{"vm"}}, Workspace: workspace, Metadata: filepath.Join(workspace, ".wm"), Source: filepath.Join(workspace, "project"), GitRemote: upstream}
	if err := os.MkdirAll(c.Metadata, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := SaveContext(c); err != nil {
		t.Fatal(err)
	}
	if err := prepareGit(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if got := git("-C", c.Source, "rev-parse", "HEAD"); got != git("-C", seed, "rev-parse", "HEAD") {
		t.Fatal("private clone did not receive the source objects")
	}
	if err := os.WriteFile(filepath.Join(c.Source, "file.txt"), []byte("remote edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("-C", c.Source, "add", ".")
	git("-C", c.Source, "commit", "-m", "remote commit")
	// A local pre-push hook must not execute when moving objects for a VM.
	marker := filepath.Join(root, "hook-ran")
	hook := filepath.Join(gitMirror(c), "hooks", "pre-push")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch "+quote(marker)+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	run := func(args ...string) {
		t.Helper()
		request := invocation{Args: args, CWD: c.Source}
		if err := validateGit(c, request); err != nil {
			t.Fatal(err)
		}
		if err := runGit(context.Background(), c, request, &output, &output); err != nil {
			t.Fatalf("bridge %v: %v\n%s", args, err, &output)
		}
	}
	run("push", "origin", "HEAD:main")
	if git("--git-dir", upstream, "rev-parse", "main") != git("-C", c.Source, "rev-parse", "HEAD") {
		t.Fatal("push did not send the remote commit")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("ran a local Git hook")
	}
	git("-C", seed, "fetch", upstream, "main")
	git("-C", seed, "reset", "--hard", "FETCH_HEAD")
	if err := os.WriteFile(filepath.Join(seed, "new.txt"), []byte("upstream edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("-C", seed, "add", ".")
	git("-C", seed, "commit", "-m", "upstream commit")
	git("-C", seed, "push", upstream, "main")
	run("fetch", "origin")
	if git("-C", c.Source, "rev-parse", "refs/remotes/origin/main") != git("-C", seed, "rev-parse", "HEAD") {
		t.Fatal("fetch did not return tracking refs")
	}
	run("pull", "origin")
	if _, err := os.Stat(filepath.Join(c.Source, "new.txt")); err != nil {
		t.Fatal("pull did not fast-forward the source")
	}
	if err := validateGit(c, invocation{Args: []string{"push"}, CWD: filepath.Dir(c.Source)}); err == nil {
		t.Fatal("delegated Git outside the context checkout")
	}
}
