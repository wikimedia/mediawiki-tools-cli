package devremote

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/releaseversion"
)

//go:embed cache.sh
var cacheScript string

// CacheEntry identifies a recognized, unused download in a local or remote cache.
type CacheEntry struct{ Location, Path string }

func sessionRelease() string {
	if tag := exactReleaseTag(cli.VersionDetails); tag != "" {
		return tag
	}
	return "development"
}

func localBinaryCache() (string, error) {
	root, err := os.UserCacheDir()
	return filepath.Join(root, "a-wikimedia-cli", "remote-binaries"), err
}

func localCacheCandidates(root string, keep map[string]bool) ([]CacheEntry, error) {
	versions, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []CacheEntry
	for _, version := range versions {
		parsed, err := releaseversion.Parse(version.Name())
		if err != nil || !parsed.Stable() || strings.ContainsAny(parsed.String(), "+-") || !strings.HasPrefix(version.Name(), "v") || !version.IsDir() || keep[version.Name()] {
			continue
		}
		architectures, err := os.ReadDir(filepath.Join(root, version.Name()))
		if err != nil {
			return nil, err
		}
		for _, arch := range architectures {
			if !arch.IsDir() || (arch.Name() != "linux-amd64" && arch.Name() != "linux-arm64") {
				continue
			}
			result = append(result, CacheEntry{"local", filepath.Join(root, version.Name(), arch.Name())})
		}
	}
	return result, nil
}

func cacheKeep(ctx context.Context) (map[string]bool, bool, error) {
	keep := map[string]bool{sessionRelease(): true}
	contexts, err := Contexts()
	if err != nil {
		return nil, false, err
	}
	for _, c := range contexts {
		if controlRequest(ctx, c.Context, "/status", nil) != nil {
			continue
		}
		state, err := loadSession(c.Context)
		if err != nil {
			return nil, false, err
		}
		if state.BinaryRelease == "" {
			return keep, true, nil
		}
		keep[state.BinaryRelease] = true
	}
	return keep, false, nil
}

// UnusedCaches inventories only recognized, unreferenced CLI cache entries.
// Supplying SSH arguments also inspects shared caches on that VM.
func UnusedCaches(ctx context.Context, ssh []string) ([]CacheEntry, error) {
	keep, unknown, err := cacheKeep(ctx)
	if err != nil {
		return nil, err
	}
	root, err := localBinaryCache()
	if err != nil {
		return nil, err
	}
	var entries []CacheEntry
	if !unknown {
		entries, err = localCacheCandidates(root, keep)
		if err != nil {
			return nil, err
		}
	}
	if len(ssh) > 0 {
		remote, err := remoteCaches(ctx, ssh, "scan", nil)
		if err != nil {
			return nil, err
		}
		entries = append(entries, remote...)
	}
	return entries, nil
}

func remoteCaches(ctx context.Context, ssh []string, action string, approved []string) ([]CacheEntry, error) {
	t := transport{args: ssh}
	var output strings.Builder
	if err := t.script(ctx, cacheScript, append([]string{action}, approved...), &output); err != nil {
		return nil, err
	}
	var entries []CacheEntry
	for _, path := range strings.Split(output.String(), "\x00") {
		if path != "" {
			entries = append(entries, CacheEntry{"remote", path})
		}
	}
	return entries, nil
}

// CleanupCaches rechecks the preview and removes only entries explicitly approved.
func CleanupCaches(ctx context.Context, ssh []string, approved []CacheEntry) (int, error) {
	current, err := UnusedCaches(ctx, ssh)
	if err != nil {
		return 0, err
	}
	eligible := map[CacheEntry]bool{}
	for _, entry := range current {
		eligible[entry] = true
	}
	removed := 0
	var remote []string
	for _, entry := range approved {
		if !eligible[entry] {
			continue
		}
		if entry.Location == "remote" {
			remote = append(remote, entry.Path)
			continue
		}
		// The downloader takes the same architecture lock before publishing a cache entry.
		lock := flock.New(entry.Path + ".lock")
		locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
		if err != nil {
			return removed, err
		}
		if !locked {
			continue
		}
		err = os.RemoveAll(entry.Path)
		lock.Unlock()
		if err != nil {
			return removed, fmt.Errorf("remove cache %s: %w", entry.Path, err)
		}
		removed++
	}
	if len(remote) > 0 {
		deleted, err := remoteCaches(ctx, ssh, "clean", remote)
		removed += len(deleted)
		if err != nil {
			return removed, err
		}
	}
	return removed, nil
}
