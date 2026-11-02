package devremote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func registryForTest(t *testing.T) {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
}

func TestContextNamesAndTargetIsolation(t *testing.T) {
	registryForTest(t)
	seen := map[string]bool{}
	for i := 0; i < 24; i++ {
		name, err := NewContextName()
		if err != nil {
			t.Fatal(err)
		}
		if seen[name] || len(strings.Split(name, "-")) != 2 {
			t.Fatalf("invalid generated name %q", name)
		}
		seen[name] = true
		c := Context{Options: Options{Context: name, SSHArgs: []string{"vm"}}}
		if err := SaveContext(c); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadContext(name)
		if err != nil || loaded.Context != name {
			t.Fatalf("load: %+v, %v", loaded, err)
		}
		c.SSHArgs = []string{"other-vm"}
		if SaveContext(c) == nil {
			t.Fatal("overwrote another context's target")
		}
	}
	if _, err := LoadContext("../escape"); err == nil {
		t.Fatal("accepted path traversal")
	}
}

func TestRoutingPreservesCommandBoundary(t *testing.T) {
	t.Setenv("CONTEXT", "")
	for _, tc := range []struct {
		args       []string
		root, name string
		control    bool
	}{
		{[]string{"dev", "-c", "calm-otter", "dc", "ps"}, "dev", "calm-otter", false},
		{[]string{"--no-interaction", "dev", "dc", "ps", "--context=calm-otter"}, "dev", "calm-otter", false},
		{[]string{"dev", "remote", "open", "--ssh", "vm"}, "dev", "default", true},
		{[]string{"dev", "-ccalm-otter", "exec", "--", "--context", "wrong"}, "dev", "calm-otter", false},
		{[]string{"dev", "--help"}, "dev", "default", true},
	} {
		root, name, control := routeArguments(tc.args)
		if root != tc.root || name != tc.name || control != tc.control {
			t.Fatalf("%v: %q %q %v", tc.args, root, name, control)
		}
	}
}

func fakeSSH(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake Linux SSH transport")
	}
	directory := t.TempDir()
	script := `#!/bin/bash
set -eu
while (($#)); do
 if [[ $1 == vm ]]; then shift; break; fi
 shift
done
exec /bin/bash -c "$*"
`
	if err := os.WriteFile(filepath.Join(directory, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSourceReplicationProtectsSecretsAndRetainsOverwrittenWork(t *testing.T) {
	registryForTest(t)
	fakeSSH(t)
	if _, err := osexec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	root := t.TempDir()
	local := filepath.Join(root, "local")
	workspace := filepath.Join(root, "remote")
	c := Context{Options: Options{Context: "calm-otter", SSHArgs: []string{"vm"}}, Workspace: workspace, Metadata: filepath.Join(workspace, ".wm"), Source: filepath.Join(workspace, "project"), Directory: local}
	write := func(name, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(local, "main.txt"), "original")
	write(filepath.Join(local, "remove.txt"), "remove me")
	write(filepath.Join(local, ".env"), "secret")
	write(filepath.Join(local, ".git", "config"), "credential")
	write(filepath.Join(c.Source, "vendor", "keep.txt"), "remote dependency")
	if err := syncDirectory(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(c.Source, "main.txt"), "remote edit")
	write(filepath.Join(local, "main.txt"), "local edit")
	if err := os.Remove(filepath.Join(local, "remove.txt")); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectory(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{filepath.Join(c.Source, "main.txt"): "local edit", filepath.Join(c.Metadata, "sync-backup", "main.txt"): "remote edit", filepath.Join(c.Metadata, "sync-backup", "remove.txt"): "remove me", filepath.Join(c.Source, "vendor", "keep.txt"): "remote dependency"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v", name, data, err)
		}
	}
	for _, name := range []string{".env", ".git/config", "remove.txt"} {
		if _, err := os.Stat(filepath.Join(c.Source, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected replicated file %s", name)
		}
	}
}

func TestLocalDevRoutingUsesSSHAndPreservesExitCode(t *testing.T) {
	registryForTest(t)
	fakeSSH(t)
	root := t.TempDir()
	metadata := filepath.Join(root, ".wm")
	if err := os.MkdirAll(filepath.Join(metadata, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata, "bin", "mw"), []byte("#!/bin/sh\nprintf 'remote output\\n'\nprintf 'remote diagnostic\\n' >&2\nexit 42\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	c := Context{Options: Options{Context: "calm-otter", SSHArgs: []string{"vm"}}, Workspace: root, Metadata: metadata, Source: filepath.Join(root, "mediawiki")}
	if err := SaveContext(c); err != nil {
		t.Fatal(err)
	}
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	var stdout, stderr bytes.Buffer
	handled, code := Early([]string{"dev", "--context", "calm-otter", "dc", "ps"}, stdin, &stdout, &stderr)
	if !handled || code != 42 || stdout.String() != "remote output\n" || stderr.String() != "remote diagnostic\n" {
		t.Fatalf("handled=%v code=%d stdout=%q stderr=%q", handled, code, &stdout, &stderr)
	}
	if handled, _ := Early([]string{"dev", "--context", "local", "dc", "ps"}, stdin, io.Discard, io.Discard); handled {
		t.Fatal("captured local context")
	}
}

func TestBridgeCannotControlSessionOrReadLocalFiles(t *testing.T) {
	registryForTest(t)
	c := Context{Options: Options{Context: "calm-otter", SSHArgs: []string{"vm"}}, Forward: []string{"phab"}}
	if err := SaveContext(c); err != nil {
		t.Fatal(err)
	}
	previous := ValidateInvocation
	defer func() { ValidateInvocation = previous }()
	ValidateInvocation = func(args []string) error { return fmt.Errorf("denied: %s", strings.Join(args, " ")) }
	for _, endpoint := range []string{"/stop", "/sync", "/invoke"} {
		data, _ := json.Marshal(invocation{Args: []string{"config", "secrets", "list"}})
		request := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
		response := httptest.NewRecorder()
		bridgeHandler(c.Context).ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s returned %d", endpoint, response.Code)
		}
	}
}
