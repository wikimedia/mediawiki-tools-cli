// Package diskcache provides typed, persistent stale-while-revalidate caches
// for public API data. It does not store credentials or HTTP responses.
package diskcache

import (
	"context"
	"encoding/json"
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
		if !time.Now().Before(cached.RefreshAfter) {
			startRefresh()
		}
		return *cached.Value, nil
	}
	return c.Refresh(ctx, fetch)
}

// Refresh fetches synchronously, replacing the cache only on success. Failed
// refreshes retain the last good value and delay automatic retries one minute.
// Explicit callers receive the fetch error even if old data remains available.
func (c *Cache[T]) Refresh(ctx context.Context, fetch func(context.Context) (T, error)) (T, error) {
	value, err := fetch(ctx)
	if err != nil {
		if cached, ok := c.read(); ok {
			cached.RefreshAfter = time.Now().Add(time.Minute)
			c.write(cached)
		}
		return value, err
	}
	c.write(entry[T]{Value: &value, RefreshAfter: time.Now().Add(c.ttl)})
	return value, nil
}

func (c *Cache[T]) read() (entry[T], bool) {
	file, err := os.Open(c.path)
	if err != nil {
		return entry[T]{}, false
	}
	defer file.Close()
	var cached entry[T]
	if json.NewDecoder(io.LimitReader(file, 4*1024*1024)).Decode(&cached) != nil || cached.Value == nil {
		return entry[T]{}, false
	}
	return cached, true
}

func (c *Cache[T]) write(cached entry[T]) {
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

// StartRefresh starts the current executable with args, detached from terminal
// streams. A per-cache directory lock suppresses duplicate workers. Locks left
// by a crashed worker expire after a minute. The worker must bound its fetch to
// less than a minute and defer FinishRefresh, using the same cache path.
func (c *Cache[T]) StartRefresh(args ...string) {
	lock := c.path + ".refresh"
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return
	}
	if info, err := os.Stat(lock); err == nil && time.Since(info.ModTime()) > time.Minute {
		_ = os.Remove(lock)
	}
	if err := os.Mkdir(lock, 0o700); err != nil {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		_ = os.Remove(lock)
		return
	}
	command := exec.Command(executable, args...)
	if err := command.Start(); err != nil {
		_ = os.Remove(lock)
		return
	}
	_ = command.Process.Release()
}

// FinishRefresh releases the background worker's lock.
func (c *Cache[T]) FinishRefresh() { _ = os.Remove(c.path + ".refresh") }
