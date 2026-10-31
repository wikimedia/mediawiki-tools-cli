// Package diskcache provides typed, persistent stale-while-revalidate caches
// for public API data. It does not store credentials or HTTP responses.
package diskcache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Cache stores one JSON value. Get returns cached data immediately, even when
// stale. Callers supply a background refresh callback appropriate to their CLI
// command; a goroutine alone cannot outlive a short-lived CLI process.
// Cache instances are immutable and may be reused concurrently.
type Cache[T any] struct {
	path string
	ttl  time.Duration
}

type entry[T any] struct {
	Value        *T        `json:"value"`
	RefreshAfter time.Time `json:"refresh_after"`
	HasValue     bool      `json:"has_value"`
	FetchError   string    `json:"fetch_error,omitempty"`
}

// New creates a disk cache with the given freshness lifetime. Cache write
// failures never prevent a successful fetch from returning its data.
func New[T any](path string, ttl time.Duration) *Cache[T] {
	return &Cache[T]{path: path, ttl: ttl}
}

// Get fetches synchronously on a missing or invalid cache. For a stale entry,
// it invokes startRefresh and returns the old value without waiting for I/O.
// startRefresh must return promptly; StartRefresh implements this for CLIs.
func (c *Cache[T]) Get(ctx context.Context, fetch func(context.Context) (T, error), startRefresh func()) (T, error) {
	if cached, ok := c.read(); ok {
		if cached.HasValue {
			if !time.Now().Before(cached.RefreshAfter) {
				startRefresh()
			}
			return cached.data(), nil
		}
		if time.Now().Before(cached.RefreshAfter) {
			var zero T
			return zero, fmt.Errorf("cache fetch temporarily backed off: %s", cached.FetchError)
		}
	}
	return c.RefreshIfStale(ctx, fetch)
}

func (e entry[T]) data() T {
	if e.Value != nil {
		return *e.Value
	}
	var zero T
	return zero
}

// Refresh forces a synchronous fetch, serialized with other refreshes. Failed
// updates retain the last good value and defer automatic retries one minute.
func (c *Cache[T]) Refresh(ctx context.Context, fetch func(context.Context) (T, error)) (T, error) {
	return c.refresh(ctx, fetch, true)
}

// RefreshIfStale serializes with other refreshes and rechecks freshness after
// acquiring the lock. Redundant background workers therefore do not refetch.
func (c *Cache[T]) RefreshIfStale(ctx context.Context, fetch func(context.Context) (T, error)) (T, error) {
	return c.refresh(ctx, fetch, false)
}

func (c *Cache[T]) refresh(ctx context.Context, fetch func(context.Context) (T, error), force bool) (T, error) {
	lock, err := c.acquire(ctx, true)
	if err != nil {
		if ctx.Err() != nil {
			var zero T
			return zero, ctx.Err()
		}
		// An unwritable cache must not prevent a successful API request.
		return fetch(ctx)
	}
	defer lock.Close()
	if !force {
		if cached, ok := c.read(); ok && time.Now().Before(cached.RefreshAfter) {
			if cached.HasValue {
				return cached.data(), nil
			}
			var zero T
			return zero, fmt.Errorf("cache fetch temporarily backed off: %s", cached.FetchError)
		}
	}
	value, err := fetch(ctx)
	if err != nil {
		cached, _ := c.read()
		cached.RefreshAfter = time.Now().Add(time.Minute)
		cached.FetchError = err.Error()
		c.write(cached)
		return value, err
	}
	c.write(entry[T]{Value: &value, HasValue: true, RefreshAfter: time.Now().Add(c.ttl)})
	return value, nil
}

func (c *Cache[T]) read() (entry[T], bool) {
	file, err := os.Open(c.path)
	if err != nil {
		return entry[T]{}, false
	}
	defer file.Close()
	var cached entry[T]
	if json.NewDecoder(io.LimitReader(file, 4*1024*1024)).Decode(&cached) != nil || (!cached.HasValue && cached.Value == nil && cached.FetchError == "") {
		return entry[T]{}, false
	}
	cached.HasValue = cached.HasValue || cached.Value != nil
	return cached, true
}

func (c *Cache[T]) write(cached entry[T]) {
	cached.HasValue = cached.HasValue || cached.Value != nil
	data, err := json.Marshal(cached)
	if err != nil || os.MkdirAll(filepath.Dir(c.path), 0o700) != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(c.path), ".mwcli-cache-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(file.Name(), c.path)
	}
}

// StartRefresh launches a bounded CLI refresh worker with null terminal streams.
// Launches are throttled for one minute under the same OS lock used by Refresh.
// No lock is transferred to the child: worker death cannot leave a held lock.
// Workers must call RefreshIfStale to skip work already completed by another
// process. The timestamp only limits launches; the OS lock protects cache writes.
func (c *Cache[T]) StartRefresh(args ...string) {
	lock, err := c.acquire(context.Background(), false)
	if err != nil {
		return
	}
	defer lock.Close()
	var nextLaunch time.Time
	if data, err := os.ReadFile(c.path + ".launch"); err == nil {
		_ = nextLaunch.UnmarshalText(data)
	}
	if time.Now().Before(nextLaunch) {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	command := exec.Command(executable, args...)
	if err := command.Start(); err != nil {
		return
	}
	data, _ := time.Now().Add(time.Minute).MarshalText()
	_ = os.WriteFile(c.path+".launch", data, 0o600)
	// Reap the child when the parent stays alive; when it exits, the child is
	// adopted by the OS and continues independently of this goroutine.
	go func() { _ = command.Wait() }()
}

func (c *Cache[T]) acquire(ctx context.Context, wait bool) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(c.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, err
		}
		locked, err := tryLockFile(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		if locked {
			return file, nil
		}
		if !wait {
			file.Close()
			return nil, fmt.Errorf("cache refresh already in progress")
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
