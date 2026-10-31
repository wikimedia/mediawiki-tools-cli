package secretstore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestFileRegistryPersistsReferencesWithoutValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "registry.json")
	r, err := NewFileRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	b := backend("test")
	s := openTest(t, "app", "test", r, b)
	if err := s.Set(t.Context(), "token", []byte("secret-fixture")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if bytes.Contains(data, []byte("secret-fixture")) {
		t.Fatal("registry contains secret")
	}
	r2, err := NewFileRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	s2 := openTest(t, "app", "test", r2, b)
	if got, err := s2.Get(t.Context(), "token"); err != nil || string(got) != "secret-fixture" {
		t.Fatal("fresh store did not resolve persisted reference")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Fatal("registry is not private")
		}
	}
}

func TestFileRegistryConcurrentStoresDoNotLoseKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "registry.json")
	b := backend("test")
	var workers sync.WaitGroup
	failures := make(chan error, 12)
	for i := range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r, err := NewFileRegistry(path)
			if err != nil {
				failures <- err
				return
			}
			s, err := Open(Config{Namespace: "app", Provider: "test", Registry: r, Backends: []Backend{b}})
			if err == nil {
				err = s.Set(t.Context(), string(rune('a'+i)), []byte("fixture"))
			}
			if err != nil {
				failures <- err
			}
		}()
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	r, err := NewFileRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := r.List(t.Context(), "app")
	if err != nil || len(entries) != 12 {
		t.Fatalf("lost registry entries: %d, %v", len(entries), err)
	}
}

func TestFileRegistryRefusesUnsafeFilesAndHonorsKeyLocks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "registry.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileRegistry(path); !errors.Is(err, ErrDenied) {
		t.Fatal("symlink registry accepted")
	}
	_ = os.Remove(path)
	r, err := NewFileRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := r.Lock(t.Context(), "app", "key")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()
	if _, err := r.Lock(ctx, "app", "key"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("second key lock did not cancel")
	}
}
