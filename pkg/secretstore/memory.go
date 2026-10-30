package secretstore

import (
	"context"
	"sync"
)

// MemoryRegistry is useful for embedded integrations and tests. It is not
// durable; use FileRegistry or an application's existing metadata registry.
type MemoryRegistry struct {
	mu      sync.Mutex
	entries map[[2]string]Entry
	locks   map[[2]string]chan struct{}
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{entries: map[[2]string]Entry{}, locks: map[[2]string]chan struct{}{}}
}
func (r *MemoryRegistry) Get(ctx context.Context, namespace, key string) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[[2]string{namespace, key}]
	if !ok {
		return Entry{}, ErrNotFound
	}
	return cloneEntry(e), nil
}
func (r *MemoryRegistry) Put(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[[2]string{entry.Namespace, entry.Key}] = cloneEntry(entry)
	return nil
}
func (r *MemoryRegistry) Delete(ctx context.Context, namespace, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, [2]string{namespace, key})
	return nil
}
func (r *MemoryRegistry) List(ctx context.Context, namespace string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := []Entry{}
	for _, entry := range r.entries {
		if entry.Namespace == namespace {
			entries = append(entries, cloneEntry(entry))
		}
	}
	return entries, nil
}
func (r *MemoryRegistry) Lock(ctx context.Context, namespace, key string) (func(), error) {
	r.mu.Lock()
	id := [2]string{namespace, key}
	lock, ok := r.locks[id]
	if !ok {
		lock = make(chan struct{}, 1)
		r.locks[id] = lock
	}
	r.mu.Unlock()
	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
