package mediawiki

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSQLitePermissionsLimitedToDatabaseFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command used inside Linux PHP container")
	}
	dir := t.TempDir()
	for _, name := range []string{"wiki.sqlite", "wiki_l10n_cache.sqlite", "wiki_jobqueue.sqlite", "wikicache.sqlite", "private.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.sqlite")
	if err := os.WriteFile(outside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.sqlite")); err != nil {
		t.Fatal(err)
	}
	command := sqlitePermissionsCommand()
	command[1] = dir
	if output, err := osexec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", output, err)
	}
	for _, name := range []string{"wiki.sqlite", "wiki_l10n_cache.sqlite", "wiki_jobqueue.sqlite", "wikicache.sqlite"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0o666 {
			t.Fatalf("%s permissions: %v %v", name, info, err)
		}
	}
	for _, name := range []string{filepath.Join(dir, "private.txt"), outside} {
		info, err := os.Stat(name)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("changed unrelated file %s", name)
		}
	}
}
