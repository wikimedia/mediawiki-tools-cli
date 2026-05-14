package phabricator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	ttlDefault      = 14 * 24 * time.Hour // 2 weeks: users, projects
	ttlTransactions = 5 * time.Minute     // 5 min: transactions
	ttlColumns      = time.Hour           // 1 hour: columns
)

type cacheEntry struct {
	Value   json.RawMessage `json:"value"`
	Expires int64           `json:"expires"`
}

type cacheStore struct {
	mu    sync.Mutex
	path  string
	data  map[string]cacheEntry
	dirty bool
}

func loadCacheStore(path string) *cacheStore {
	cs := &cacheStore{path: path, data: make(map[string]cacheEntry)}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cs.data)
	}
	return cs
}

func (cs *cacheStore) get(key string, dst interface{}) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	entry, ok := cs.data[key]
	if !ok || entry.Expires < time.Now().Unix() {
		return false
	}
	return json.Unmarshal(entry.Value, dst) == nil
}

func (cs *cacheStore) set(key string, value interface{}, ttl time.Duration) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.data[key] = cacheEntry{
		Value:   json.RawMessage(data),
		Expires: time.Now().Add(ttl).Unix(),
	}
	cs.dirty = true
}

func (cs *cacheStore) flush() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if !cs.dirty {
		return
	}
	_ = os.MkdirAll(filepath.Dir(cs.path), 0o700)
	data, err := json.MarshalIndent(cs.data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(cs.path, data, 0o600)
	cs.dirty = false
}

// PhabCache manages local cache files for phids and transactions.
type PhabCache struct {
	phids        *cacheStore // users, projects, columns, task-number→phid
	transactions *cacheStore
}

func newPhabCache(dir string) *PhabCache {
	return &PhabCache{
		phids:        loadCacheStore(filepath.Join(dir, "phids.json")),
		transactions: loadCacheStore(filepath.Join(dir, "transactions.json")),
	}
}

func (pc *PhabCache) flush() {
	pc.phids.flush()
	pc.transactions.flush()
}
