package diskcache

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

func TestColdFailureBacksOffAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.json")
	cache := New[int](path, time.Hour)
	calls := 0
	fail := func(context.Context) (int, error) { calls++; return 0, errors.New("upstream down") }
	if _, err := cache.Get(t.Context(), fail, func() { t.Fatal("cold background refresh") }); err == nil {
		t.Fatal("missing failure")
	}
	next := New[int](path, time.Hour)
	if _, err := next.Get(t.Context(), fail, func() { t.Fatal("negative entry refreshed in background") }); err == nil || calls != 1 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
	// Forced refresh bypasses the backoff and replaces the negative entry.
	if _, err := next.Refresh(t.Context(), func(context.Context) (int, error) { return 7, nil }); err != nil {
		t.Fatal(err)
	}
	got, err := next.Get(t.Context(), fail, func() { t.Fatal("fresh cache refreshed") })
	if got != 7 || err != nil {
		t.Fatalf("got=%d error=%v", got, err)
	}
}

func TestExpiredColdFailureRetries(t *testing.T) {
	cache := New[int](filepath.Join(t.TempDir(), "value.json"), time.Hour)
	cache.write(entry[int]{FetchError: "old failure", RefreshAfter: time.Now().Add(-time.Minute)})
	got, err := cache.Get(t.Context(), func(context.Context) (int, error) { return 9, nil }, func() { t.Fatal("negative entry returned as stale data") })
	if got != 9 || err != nil {
		t.Fatalf("got=%d error=%v", got, err)
	}
}

func TestNilValuesSurviveDiskRoundTrip(t *testing.T) {
	cache := New[[]string](filepath.Join(t.TempDir(), "nil.json"), time.Hour)
	if _, err := cache.Refresh(t.Context(), func(context.Context) ([]string, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	next := New[[]string](cache.path, time.Hour)
	got, err := next.Get(t.Context(), func(context.Context) ([]string, error) { t.Fatal("cached nil was refetched"); return nil, nil }, func() { t.Fatal("fresh nil was refreshed") })
	if got != nil || err != nil {
		t.Fatalf("got=%v error=%v", got, err)
	}
}

func TestRefreshesSerializeAndRecheck(t *testing.T) {
	cache := New[string](filepath.Join(t.TempDir(), "value.json"), time.Hour)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := cache.Refresh(t.Context(), func(context.Context) (string, error) { close(entered); <-release; return "first", nil })
		done <- err
	}()
	<-entered
	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := cache.Refresh(t.Context(), func(context.Context) (string, error) { return "second", nil })
		secondDone <- err
	}()
	<-secondStarted
	// The first request cannot overwrite the second because fetch and write
	// remain within the same cross-process lock.
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	got, err := cache.RefreshIfStale(t.Context(), func(context.Context) (string, error) { t.Fatal("fresh data fetched again"); return "", nil })
	if got != "second" || err != nil {
		t.Fatalf("got=%q error=%v", got, err)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for subprocess")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCacheProcessHelper(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	mode, path := os.Args[index+1], os.Args[index+2]
	cache := New[int](path, time.Hour)
	switch mode {
	case "lock":
		lock, err := cache.acquire(t.Context(), true)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := os.WriteFile(path+".ready", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Second)
	case "launch":
		cache.StartRefresh("-test.run=^TestCacheProcessHelper$", "--", "refresh", path)
	case "refresh":
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_, err := cache.RefreshIfStale(ctx, func(ctx context.Context) (int, error) {
			f, err := os.OpenFile(path+".fetches", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return 0, err
			}
			_, err = f.WriteString("fetch\n")
			f.Close()
			if err != nil {
				return 0, err
			}
			for {
				if _, err := os.Stat(path + ".gate"); err == nil {
					break
				}
				select {
				case <-ctx.Done():
					return 0, ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
			return 8, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+".done", nil, 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unknown helper mode")
	}
}

func helper(mode, path string) *exec.Cmd {
	return exec.Command(os.Args[0], "-test.run=^TestCacheProcessHelper$", "--", mode, path)
}

func TestLockReleasedWhenWorkerKilled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	process := helper("lock", path)
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = process.Process.Kill(); _ = process.Wait() }()
	waitFor(t, func() bool { _, err := os.Stat(path + ".ready"); return err == nil })
	cache := New[int](path, time.Hour)
	if lock, err := cache.acquire(t.Context(), false); err == nil {
		lock.Close()
		t.Fatal("acquired another process's lock")
	}
	if err := process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.Wait()
	lock, err := cache.acquire(t.Context(), false)
	if err != nil {
		t.Fatal("dead worker retained lock", err)
	}
	lock.Close()
}

func TestBackgroundRefreshSurvivesParentAndSuppressesDuplicateLaunch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	cache := New[int](path, time.Hour)
	value := 1
	cache.write(entry[int]{Value: &value, RefreshAfter: time.Now().Add(-time.Hour)})
	if out, err := helper("launch", path).CombinedOutput(); err != nil {
		t.Fatalf("launcher failed: %s %v", out, err)
	}
	// The launcher has exited. Its worker still waits for the gate.
	waitFor(t, func() bool { _, err := os.Stat(path + ".fetches"); return err == nil })
	for i := 0; i < 3; i++ {
		cache.StartRefresh("-test.run=^TestCacheProcessHelper$", "--", "refresh", path)
	}
	if err := os.WriteFile(path+".gate", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { _, err := os.Stat(path + ".done"); return err == nil })
	cached, ok := cache.read()
	if !ok || cached.data() != 8 {
		t.Fatalf("cache=%+v", cached)
	}
	fetches, err := os.ReadFile(path + ".fetches")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(fetches), "fetch\n") != 1 {
		t.Fatalf("duplicate fetches: %s", fetches)
	}
}

func TestRedundantProcessRefreshRechecksFreshness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	first := helper("refresh", path)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Process.Kill(); _ = first.Wait() }()
	waitFor(t, func() bool { _, err := os.Stat(path + ".fetches"); return err == nil })
	second := helper("refresh", path)
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Process.Kill(); _ = second.Wait() }()
	if err := os.WriteFile(path+".gate", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := second.Wait(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path + ".fetches")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fetch\n" {
		t.Fatalf("redundant worker fetched again: %s", data)
	}
}
