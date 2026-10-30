package secretstore

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
)

// FileRegistry stores only references. Files are private, replaced atomically,
// and guarded by OS locks across processes; a crashed process releases its lock.
type FileRegistry struct{ path string }

func NewFileRegistry(path string) (*FileRegistry, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrInvalid
	}
	r := &FileRegistry{path: path}
	if err := r.check(); err != nil {
		return nil, SafeError("open", "registry", err)
	}
	return r, nil
}
func (r *FileRegistry) check() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(filepath.Dir(r.path))
	if err != nil {
		return err
	}
	if !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return ErrDenied
	}
	info, err = os.Lstat(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return ErrDenied
	}
	return nil
}
func (r *FileRegistry) load() (map[[2]string]Entry, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	entries := map[[2]string]Entry{}
	file, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 4<<20 {
		return nil, ErrTooLarge
	}
	var document struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	if err := decoder.Decode(&document); err != nil || document.Version != 1 {
		return nil, ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, ErrInvalid
	}
	for _, entry := range document.Entries {
		if !valid(entry.Namespace) || !valid(entry.Key) || !valid(entry.Reference.Provider) {
			return nil, ErrInvalid
		}
		id := [2]string{entry.Namespace, entry.Key}
		if _, ok := entries[id]; ok {
			return nil, ErrInvalid
		}
		entries[id] = entry
	}
	return entries, nil
}
func (r *FileRegistry) Get(ctx context.Context, namespace, key string) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	entries, err := r.load()
	if err != nil {
		return Entry{}, SafeError("read", "registry", err)
	}
	entry, ok := entries[[2]string{namespace, key}]
	if !ok {
		return Entry{}, ErrNotFound
	}
	return entry, nil
}
func (r *FileRegistry) List(ctx context.Context, namespace string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := r.load()
	if err != nil {
		return nil, SafeError("read", "registry", err)
	}
	result := []Entry{}
	for _, entry := range entries {
		if entry.Namespace == namespace {
			result = append(result, entry)
		}
	}
	return result, nil
}
func (r *FileRegistry) Put(ctx context.Context, entry Entry) error {
	if !valid(entry.Namespace) || !valid(entry.Key) || !valid(entry.Reference.Provider) {
		return ErrInvalid
	}
	return r.mutate(ctx, func(entries map[[2]string]Entry) { entries[[2]string{entry.Namespace, entry.Key}] = entry })
}
func (r *FileRegistry) Delete(ctx context.Context, namespace, key string) error {
	return r.mutate(ctx, func(entries map[[2]string]Entry) { delete(entries, [2]string{namespace, key}) })
}
func (r *FileRegistry) mutate(ctx context.Context, mutate func(map[[2]string]Entry)) error {
	unlock, err := r.lockFile(ctx, r.path+".lock")
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := r.load()
	if err != nil {
		return SafeError("read", "registry", err)
	}
	mutate(entries)
	list := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, entry)
	}
	data, err := json.Marshal(struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}{1, list})
	if err != nil || len(data) > 4<<20 {
		return ErrTooLarge
	}
	file, err := os.CreateTemp(filepath.Dir(r.path), ".secretstore-*")
	if err != nil {
		return SafeError("write", "registry", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return SafeError("write", "registry", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return SafeError("sync", "registry", err)
	}
	if err := file.Close(); err != nil {
		return SafeError("close", "registry", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return SafeError("replace", "registry", os.Rename(file.Name(), r.path))
}
func (r *FileRegistry) Lock(ctx context.Context, namespace, key string) (func(), error) {
	if !valid(namespace) || !valid(key) {
		return nil, ErrInvalid
	}
	hash := sha256.Sum256([]byte(namespace + "\x00" + key))
	return r.lockFile(ctx, fmt.Sprintf("%s.%x.lock", r.path, hash))
}
func (r *FileRegistry) lockFile(ctx context.Context, path string) (func(), error) {
	if err := r.check(); err != nil {
		return nil, SafeError("lock", "registry", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, ErrDenied
		}
	} else if !os.IsNotExist(err) {
		return nil, SafeError("lock", "registry", err)
	}
	lock := flock.New(path)
	ok, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return nil, SafeError("lock", "registry", err)
	}
	if !ok {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, ErrUnavailable
	}
	return func() { _ = lock.Unlock() }, nil
}
