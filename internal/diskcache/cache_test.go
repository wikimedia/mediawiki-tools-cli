package diskcache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestColdThenWarmCache(t *testing.T) {
	cache := New[[]string](filepath.Join(t.TempDir(), "cache", "names.json"), 24*time.Hour)
	calls := 0
	fetch := func(context.Context) ([]string, error) { calls++; return []string{"alpha", "zeta"}, nil }
	for i := 0; i < 2; i++ {
		names, err := cache.Get(t.Context(), fetch, func() { t.Fatal("fresh data caused refresh") })
		if err != nil || !reflect.DeepEqual(names, []string{"alpha", "zeta"}) {
			t.Fatalf("names=%v error=%v", names, err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetches=%d", calls)
	}
	// A new instance sees the same disk data across CLI invocations.
	next := New[[]string](cache.path, 24*time.Hour)
	if _, err := next.Get(t.Context(), fetch, func() { t.Fatal("fresh cache refreshed") }); err != nil || calls != 1 {
		t.Fatalf("fetches=%d error=%v", calls, err)
	}
}

func TestStaleReturnsWithoutFetch(t *testing.T) {
	cache := New[int](filepath.Join(t.TempDir(), "count.json"), time.Hour)
	value := 42
	cache.write(entry[int]{Value: &value, RefreshAfter: time.Now().Add(-time.Hour)})
	started := 0
	got, err := cache.Get(t.Context(), func(context.Context) (int, error) { t.Fatal("stale read blocked on fetch"); return 0, nil }, func() { started++ })
	if got != 42 || err != nil || started != 1 {
		t.Fatalf("value=%d error=%v refreshes=%d", got, err, started)
	}
}

func TestFailedRefreshPreservesValueAndBacksOff(t *testing.T) {
	cache := New[string](filepath.Join(t.TempDir(), "value.json"), time.Hour)
	value := "old"
	cache.write(entry[string]{Value: &value, RefreshAfter: time.Now().Add(-time.Hour)})
	failed := func(context.Context) (string, error) { return "", errors.New("upstream unavailable") }
	if _, err := cache.Refresh(t.Context(), failed); err == nil {
		t.Fatal("explicit refresh hid error")
	}
	got, err := cache.Get(t.Context(), failed, func() { t.Fatal("failed refresh wasn't backed off") })
	if got != "old" || err != nil {
		t.Fatalf("value=%q error=%v", got, err)
	}
}

func TestCorruptAndIncompleteCacheRefetched(t *testing.T) {
	for _, data := range []string{"broken", `{}`, `{"value":null}`, `{"value":"wrong type"}`} {
		t.Run(data, func(t *testing.T) {
			cache := New[int](filepath.Join(t.TempDir(), "value.json"), time.Hour)
			if err := os.WriteFile(cache.path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := cache.Get(t.Context(), func(context.Context) (int, error) { return 7, nil }, func() { t.Fatal("invalid data used") })
			if got != 7 || err != nil {
				t.Fatalf("value=%d error=%v", got, err)
			}
		})
	}
}

func TestUnwritableCacheDoesNotBlockFetch(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := New[int](filepath.Join(parent, "value.json"), time.Hour)
	got, err := cache.Refresh(t.Context(), func(context.Context) (int, error) { return 3, nil })
	if got != 3 || err != nil {
		t.Fatalf("value=%d error=%v", got, err)
	}
}

func TestExistingRefreshLockSuppressesWorker(t *testing.T) {
	cache := New[int](filepath.Join(t.TempDir(), "value.json"), time.Hour)
	lock := cache.path + ".refresh"
	if err := os.Mkdir(lock, 0o700); err != nil {
		t.Fatal(err)
	}
	// The existing lock remains owned by the original worker. The argument
	// is safe even if a regression accidentally starts the test executable.
	cache.StartRefresh("-test.run=^$")
	if _, err := os.Stat(lock); err != nil {
		t.Fatal("lock was removed by duplicate refresh")
	}
	cache.FinishRefresh()
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatalf("lock wasn't released: %v", err)
	}
}
