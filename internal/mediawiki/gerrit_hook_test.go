package mediawiki

import (
	"net/http"
	"net/http/httptest"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func hookTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := osexec.Command("git", "init", dir)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "empty"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", output, err)
	}
	return dir
}

func TestInstallGerritCommitHook(t *testing.T) {
	previousVersion := cli.VersionDetails.Version
	cli.VersionDetails.Version = cli.Version("1.2.3")
	t.Cleanup(func() { cli.VersionDetails.Version = previousVersion })
	const wantUserAgent = "a-wikimedia-cli/1.2.3 (https://www.mediawiki.org/wiki/Cli)"
	for _, tc := range []struct {
		name, body string
		code       int
		wantErr    bool
	}{
		{"script", "#!/bin/sh\nexit 0\n", 200, false},
		{"server error", "error", 503, true},
		{"html", "<html>not a hook</html>", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := hookTestRepo(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.UserAgent(); got != wantUserAgent {
					t.Errorf("User-Agent = %q, want %q", got, wantUserAgent)
				}
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			err := installGerritCommitHook(dir, server.URL)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			path := filepath.Join(dir, ".git", "hooks", "commit-msg")
			data, readErr := os.ReadFile(path)
			if tc.wantErr {
				if !os.IsNotExist(readErr) {
					t.Fatalf("failed download left a hook: %v", readErr)
				}
				return
			}
			if readErr != nil || string(data) != tc.body {
				t.Fatalf("hook=%q err=%v", data, readErr)
			}
			info, _ := os.Stat(path)
			if runtime.GOOS != "windows" && info.Mode()&0o100 == 0 {
				t.Fatal("hook is not executable")
			}
			if err := installGerritCommitHook(dir, "http://127.0.0.1:1"); err != nil {
				t.Fatalf("existing hook must not trigger download: %v", err)
			}
		})
	}
}

func TestGerritHookRespectsHooksPath(t *testing.T) {
	dir := hookTestRepo(t)
	if output, err := osexec.Command("git", "-C", dir, "config", "core.hooksPath", "managed-hooks").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", output, err)
	}
	if err := installGerritCommitHook(dir, "http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "managed-hooks")); !os.IsNotExist(err) {
		t.Fatal("modified user-managed hooks")
	}
}
