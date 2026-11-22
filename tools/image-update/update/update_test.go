package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidArgumentsFailBeforeReadingData(t *testing.T) {
	for _, args := range [][]string{nil, {"old"}, {"old", "new", "dangling"}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestReplacementPreservesPermissionsAndReturnsIOErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(path, []byte("old-image\n"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := replaceInFile(path, "old-image", "new-image"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new-image\n" || info.Mode().Perm() != 0o750 {
		t.Fatalf("replacement = %q, mode %v", content, info.Mode())
	}
	if err := replaceInFile(path+"-missing", "old", "new"); err == nil {
		t.Fatal("missing input succeeded")
	}
	if err := replaceInDirectory(path, "old", "new"); err == nil {
		t.Fatal("non-directory succeeded")
	}
}
