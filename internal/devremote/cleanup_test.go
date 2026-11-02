package devremote

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeCleanupFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestForgetRemoteRetainsLocalSource(t *testing.T) {
	registryForTest(t)
	local := filepath.Join(t.TempDir(), "local-source.txt")
	writeCleanupFile(t, local, "local changes")
	c := Context{Options: Options{Context: "calm-otter", SSHArgs: []string{"offline-vm"}}, Workspace: "/home/test/wm/calm-otter", Metadata: "/home/test/wm/calm-otter/.wm", Source: "/home/test/wm/calm-otter/mediawiki", Directory: filepath.Dir(local)}
	if err := SaveContext(c); err != nil {
		t.Fatal(err)
	}
	directory, _ := contextDirectory(c.Context)
	writeCleanupFile(t, filepath.Join(directory, "git.git", "fixture"), "mirror")
	if err := RemoveContext(context.Background(), c.Context, false, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadContext(c.Context); !os.IsNotExist(err) {
		t.Fatalf("context retained: %v", err)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatal("local source was deleted")
	}
}

func remoteCleanupFixture(t *testing.T) Context {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("remote cleanup executes on Linux")
	}
	registryForTest(t)
	fakeSSH(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	workspace := filepath.Join(home, "wm", "calm-otter")
	metadata := filepath.Join(workspace, ".wm")
	log := filepath.Join(home, "teardown.log")
	t.Setenv("WM_REMOVE_TEST_LOG", log)
	stub := `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$WM_REMOVE_TEST_LOG"
if [[ ${WM_REMOVE_TEST_FAIL:-} == yes ]]; then exit 9; fi
`
	writeCleanupFile(t, filepath.Join(metadata, "bin", "mw"), stub)
	writeCleanupFile(t, filepath.Join(metadata, "context"), "calm-otter\n")
	writeCleanupFile(t, filepath.Join(metadata, "port"), "6195\n")
	writeCleanupFile(t, filepath.Join(metadata, "recipe"), "none\n")
	writeCleanupFile(t, filepath.Join(workspace, "mediawiki", "uncommitted.txt"), "remote changes")
	c := Context{Options: Options{Context: "calm-otter", SSHArgs: []string{"vm"}}, Workspace: workspace, Metadata: metadata, Source: filepath.Join(workspace, "mediawiki")}
	if err := SaveContext(c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDeleteRemoteWorkspaceTearsDownBeforeDeleting(t *testing.T) {
	c := remoteCleanupFixture(t)
	var output bytes.Buffer
	if err := RemoveContext(context.Background(), c.Context, true, &output, &output); err != nil {
		t.Fatalf("%v: %s", err, &output)
	}
	if _, err := os.Stat(c.Workspace); !os.IsNotExist(err) {
		t.Fatal("workspace retained")
	}
	log, err := os.ReadFile(os.Getenv("WM_REMOVE_TEST_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	if string(log) != "--no-interaction dev destroy\n--no-interaction dev vscode stop\n" {
		t.Fatalf("unexpected teardown: %s", log)
	}
	if _, err := LoadContext(c.Context); !os.IsNotExist(err) {
		t.Fatal("registration retained")
	}
}

func TestFailedTeardownRetainsWorkspaceAndRegistration(t *testing.T) {
	c := remoteCleanupFixture(t)
	t.Setenv("WM_REMOVE_TEST_FAIL", "yes")
	if err := RemoveContext(context.Background(), c.Context, true, io.Discard, io.Discard); err == nil {
		t.Fatal("ignored teardown failure")
	}
	if _, err := LoadContext(c.Context); err != nil {
		t.Fatal("lost recovery registration")
	}
	if _, err := os.Stat(filepath.Join(c.Source, "uncommitted.txt")); err != nil {
		t.Fatal("lost source after teardown failed")
	}
}

func TestRemoteDeletionRejectsSymlinksAndUnfamiliarLocations(t *testing.T) {
	c := remoteCleanupFixture(t)
	outside := filepath.Join(os.Getenv("HOME"), "important")
	if err := os.Rename(c.Workspace, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, c.Workspace); err != nil {
		t.Fatal(err)
	}
	if err := RemoveContext(context.Background(), c.Context, true, io.Discard, io.Discard); err == nil {
		t.Fatal("followed workspace symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "mediawiki", "uncommitted.txt")); err != nil {
		t.Fatal("deleted symlink target")
	}
	c.Workspace, c.Metadata, c.Source = outside, outside+"/.wm", outside+"/mediawiki"
	if err := SaveContext(c); err != nil {
		t.Fatal(err)
	}
	if err := RemoveContext(context.Background(), c.Context, true, io.Discard, io.Discard); err == nil {
		t.Fatal("deleted unfamiliar workspace location")
	}
}

func TestLocalCacheCleanupIsScopedToApprovedEntries(t *testing.T) {
	registryForTest(t)
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses LocalAppData for its cache")
	}
	cache, _ := localBinaryCache()
	old := filepath.Join(cache, "v0.1.0", "linux-amd64")
	newer := filepath.Join(cache, "v0.2.0", "linux-arm64")
	writeCleanupFile(t, filepath.Join(old, "wm"), "old")
	writeCleanupFile(t, filepath.Join(newer, "wm"), "newer")
	unknown := filepath.Join(cache, "my-files", "linux-amd64", "keep")
	writeCleanupFile(t, unknown, "keep")
	entries, err := UnusedCaches(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var approved []CacheEntry
	for _, entry := range entries {
		if entry.Path == old {
			approved = append(approved, entry)
		}
	}
	removed, err := CleanupCaches(context.Background(), nil, approved)
	if err != nil || removed != 1 {
		t.Fatalf("cleanup %d: %v", removed, err)
	}
	for _, path := range []string{newer, unknown} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("deleted unapproved path %s", path)
		}
	}
}

func TestRemoteCacheRetainsReferencedAndRunningDownloads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("remote cache executes on Linux")
	}
	registryForTest(t)
	fakeSSH(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := filepath.Join(os.Getenv("HOME"), ".cache", "a-wikimedia-cli")
	used := filepath.Join(root, "bin", "mw-"+strings.Repeat("a", 64))
	unused := filepath.Join(root, "bin", "mw-"+strings.Repeat("b", 64))
	running := filepath.Join(root, "bin", "mw-"+strings.Repeat("c", 64))
	for _, path := range []string{used, unused} {
		writeCleanupFile(t, path, "binary")
	}
	data, err := os.ReadFile("/bin/sleep")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(running, data, 0o700); err != nil {
		t.Fatal(err)
	}
	child := exec.Command(running, "60")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	metadata := filepath.Join(os.Getenv("HOME"), "wm", "calm-otter", ".wm")
	writeCleanupFile(t, filepath.Join(metadata, "binary"), used)
	editor := filepath.Join(root, "code-server", "1.0.0-amd64")
	stale := filepath.Join(root, "code-server", "0.9.0-amd64")
	writeCleanupFile(t, filepath.Join(editor, "bin", "code-server"), "editor")
	writeCleanupFile(t, filepath.Join(stale, "bin", "code-server"), "old editor")
	writeCleanupFile(t, filepath.Join(metadata, "editor-cache"), editor)
	entries, err := UnusedCaches(context.Background(), []string{"vm"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected unused entries: %+v", entries)
	}
	// A workspace can start using an entry after preview: cleanup must recheck it.
	writeCleanupFile(t, filepath.Join(os.Getenv("HOME"), "wm", "new-context", ".wm", "binary"), unused)
	removed, err := CleanupCaches(context.Background(), []string{"vm"}, entries)
	if err != nil || removed != 1 {
		t.Fatalf("cleanup %d: %v", removed, err)
	}
	for _, path := range []string{used, unused, running, editor} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("deleted referenced download %s", path)
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("unused editor retained")
	}
}
