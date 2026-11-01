package devenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func intentEnvironment(t *testing.T) Environment {
	t.Helper()
	previous := Context
	Context = "intent-test"
	t.Cleanup(func() { Context = previous })
	return Environment{BaseDir: t.TempDir()}
}

func TestServiceIntentAdoptionAndSnapshotMigration(t *testing.T) {
	inventory := []dockercompose.ServiceStatus{{Name: "mysql", State: "running"}, {Name: "redis", State: "exited"}, {Name: "setup", State: "running", OneShot: true}, {Name: "gateway-auth", State: "running"}, {Name: "cloudflared", State: "running"}}
	for _, snapshot := range []bool{false, true} {
		t.Run(map[bool]string{false: "no snapshot", true: "legacy snapshot"}[snapshot], func(t *testing.T) {
			m := intentEnvironment(t)
			if snapshot {
				if err := m.SaveResumeServices([]string{"redis"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.EnsureServiceIntent(inventory); err != nil {
				t.Fatal(err)
			}
			wanted, _, err := m.ResumeServices()
			expected := []string{"mysql"}
			if snapshot {
				expected = []string{"redis"}
			}
			if err != nil || !reflect.DeepEqual(wanted, expected) {
				t.Fatalf("wanted %v expected %v error %v", wanted, expected, err)
			}
			intents, err := m.ServiceIntents()
			if err != nil || len(intents) != 3 {
				t.Fatalf("intents %v error %v", intents, err)
			}
			for _, intent := range intents {
				if intent.Name == "setup" && (!intent.OneShot || intent.Desired != "stopped") {
					t.Fatal(intent)
				}
			}
			// Once migrated, live Docker status cannot overwrite explicit desired state.
			if err := m.EnsureServiceIntent([]dockercompose.ServiceStatus{{Name: "redis", State: "running"}, {Name: "mysql", State: "exited"}}); err != nil {
				t.Fatal(err)
			}
			wanted, _, err = m.ResumeServices()
			if err != nil || !reflect.DeepEqual(wanted, expected) {
				t.Fatalf("migration changed intent on retry: %v %v", wanted, err)
			}
		})
	}
}

func TestServiceActionsPreserveStoppedServicesAndNewDependencies(t *testing.T) {
	m := intentEnvironment(t)
	before := []dockercompose.ServiceStatus{{Name: "redis", State: "exited"}, {Name: "mysql", State: "running"}}
	if err := m.EnsureServiceIntent(before); err != nil {
		t.Fatal(err)
	}
	after := append(append([]dockercompose.ServiceStatus{}, before...), dockercompose.ServiceStatus{Name: "mediawiki", State: "running"}, dockercompose.ServiceStatus{Name: "traefik", State: "running"}, dockercompose.ServiceStatus{Name: "setup", State: "exited", OneShot: true})
	if err := m.recordServiceAction([]string{"mediawiki", "setup"}, ServiceStarted, before, after); err != nil {
		t.Fatal(err)
	}
	wanted, _, err := m.ResumeServices()
	if err != nil || !reflect.DeepEqual(wanted, []string{"mediawiki", "mysql", "traefik"}) {
		t.Fatalf("wanted %v error %v", wanted, err)
	}
	if err := m.recordServiceAction([]string{"mediawiki"}, ServiceStopped, after, after); err != nil {
		t.Fatal(err)
	}
	wanted, _, _ = m.ResumeServices()
	if !reflect.DeepEqual(wanted, []string{"mysql", "traefik"}) {
		t.Fatal(wanted)
	}
	if err := m.recordServiceAction([]string{"setup", "redis"}, ServiceDestroyed, after, nil); err != nil {
		t.Fatal(err)
	}
	intents, err := m.ServiceIntents()
	if err != nil || len(intents) != 3 {
		t.Fatalf("intents %v error %v", intents, err)
	}
}

func TestExplicitStopClearsIntentForMissingContainers(t *testing.T) {
	m := intentEnvironment(t)
	if err := m.SaveResumeServices([]string{"mysql"}); err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureServiceIntent(nil); err != nil {
		t.Fatal(err)
	}
	if err := m.recordServiceAction([]string{"mysql", "never-created"}, ServiceStopped, nil, nil); err != nil {
		t.Fatal(err)
	}
	intents, err := m.ServiceIntents()
	if err != nil || !reflect.DeepEqual(intents, []ServiceIntent{{Name: "mysql", Desired: "stopped"}}) {
		t.Fatalf("intents %v error %v", intents, err)
	}
}

func TestConcurrentServiceIntentMutationsDoNotLoseUpdates(t *testing.T) {
	m := intentEnvironment(t)
	if err := m.EnsureServiceIntent(nil); err != nil {
		t.Fatal(err)
	}
	names := []string{"mysql", "mediawiki", "redis", "traefik", "elastic", "jobrunner"}
	var wg sync.WaitGroup
	failures := make(chan error, len(names))
	for _, name := range names {
		wg.Add(1)
		go func(name string) { defer wg.Done(); failures <- m.UpdateResumeServices([]string{name}, true) }(name)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	wanted, _, err := m.ResumeServices()
	if err != nil || len(wanted) != len(names) {
		t.Fatalf("lost updates: %v %v", wanted, err)
	}
}

func TestLifecycleLockCancellationAndRelease(t *testing.T) {
	m := intentEnvironment(t)
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- m.WithServiceLifecycle(context.Background(), func() error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	called := false
	err := m.WithServiceLifecycle(ctx, func() error { called = true; return nil })
	close(release)
	if firstErr := <-finished; firstErr != nil {
		t.Fatal(firstErr)
	}
	if called || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock ignored cancellation: called=%v err=%v", called, err)
	}
	if err := m.WithServiceLifecycle(context.Background(), func() error { return nil }); err != nil {
		t.Fatalf("lock retained after release: %v", err)
	}
}

func TestServiceSyncPreservesLocksAndAtomicStateFiles(t *testing.T) {
	m := intentEnvironment(t)
	m.EnsureReady()
	for _, name := range []string{".service-state.lock", ".service-lifecycle.lock", ".resume-services-inflight"} {
		path := filepath.Join(m.Directory(), name)
		if err := os.WriteFile(path, []byte("runtime"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m.EnsureReady()
	for _, name := range []string{".service-state.lock", ".service-lifecycle.lock", ".resume-services-inflight"} {
		if _, err := os.Stat(filepath.Join(m.Directory(), name)); err != nil {
			t.Fatalf("sync removed %s: %v", name, err)
		}
	}
}

func TestFailedServiceOperationPreservesIntent(t *testing.T) {
	m := intentEnvironment(t)
	if err := m.SaveResumeServices([]string{"mysql"}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
 "compose") exit 0 ;;
 *"ps --all --quiet"*) echo fixture ;;
 "container inspect -- fixture") printf '%s\n' '[{"Config":{"Labels":{"com.docker.compose.service":"mysql"}},"State":{"Status":"running"}}]' ;;
 *" config") printf 'services:\n  mysql:\n    image: fixture\n' ;;
 *) echo "unexpected command: $*" >&2;exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	failure := errors.New("Docker partially failed")
	err := m.ChangeServices(context.Background(), []string{"mysql"}, ServiceStopped, func() error { return failure })
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	wanted, _, err := m.ResumeServices()
	if err != nil || !reflect.DeepEqual(wanted, []string{"mysql"}) {
		t.Fatalf("failure erased previous intent: %v %v", wanted, err)
	}
}
