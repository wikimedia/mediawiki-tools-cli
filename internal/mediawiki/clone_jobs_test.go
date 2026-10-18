package mediawiki

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloneJobsBoundAndErrors(t *testing.T) {
	var active, peak, calls atomic.Int32
	release := make(chan struct{})
	entered := make(chan struct{}, 6)
	failure := errors.New("clone failed")
	var work []func() error
	for i := 0; i < 6; i++ {
		index := i
		work = append(work, func() error {
			n := active.Add(1)
			defer active.Add(-1)
			calls.Add(1)
			for old := peak.Load(); n > old; old = peak.Load() {
				if peak.CompareAndSwap(old, n) {
					break
				}
			}
			entered <- struct{}{}
			<-release
			if index == 1 || index == 4 {
				return fmt.Errorf("repository %d: %w", index, failure)
			}
			return nil
		})
	}
	done := make(chan error, 1)
	go func() { done <- runCloneJobs(work, 2) }()
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("workers did not start")
		}
	}
	if peak.Load() != 2 {
		t.Fatalf("peak %d", peak.Load())
	}
	close(release)
	err := <-done
	if calls.Load() != 6 || peak.Load() > 2 || !errors.Is(err, failure) {
		t.Fatalf("calls=%d peak=%d err=%v", calls.Load(), peak.Load(), err)
	}
	if err.Error() != "repository 1: clone failed\nrepository 4: clone failed" {
		t.Fatal(err)
	}
}

func TestCloneJobsEmptyAndInvalid(t *testing.T) {
	if err := runCloneJobs(nil, 4); err != nil {
		t.Fatal(err)
	}
	if err := runCloneJobs(nil, 0); err == nil {
		t.Fatal("accepted invalid jobs")
	}
	err := MediaWiki(t.TempDir()).CloneSetup(CloneOpts{GerritInteractionType: "ssh", Jobs: 2})
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("missing validation: %v", err)
	}
}

func TestCloneLocalRepoAndSetRemote(t *testing.T) {
	repo := hookTestRepo(t)
	destination := filepath.Join(t.TempDir(), "clone")
	if err := cloneAndSetRemote(destination, repo, "https://example.invalid/review.git", false, false, false, false); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("git", "-C", destination, "remote", "get-url", "origin").Output()
	if err != nil || strings.TrimSpace(string(output)) != "https://example.invalid/review.git" {
		t.Fatalf("remote=%q err=%v", output, err)
	}
	if err := cloneAndSetRemote(destination, repo, repo, false, false, false, false); err == nil {
		t.Fatal("clone failure was swallowed")
	}
}
