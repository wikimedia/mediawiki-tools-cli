package dev

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

type fakeLifecycleProject struct {
	running, exited, created, started, stopped, restarted []string
	failure                                               error
	stopFailure                                           error
	calls                                                 int
}

func (p *fakeLifecycleProject) ServicesWithStatus(status string) ([]string, error) {
	if p.failure != nil {
		return nil, p.failure
	}
	switch status {
	case "running":
		return p.running, nil
	case "exited":
		return p.exited, nil
	case "created":
		return p.created, nil
	}
	panic(status)
}

func (p *fakeLifecycleProject) StartContainers(s []string) error {
	p.calls++
	p.started = append([]string{}, s...)
	return p.failure
}

func (p *fakeLifecycleProject) Stop(s []string) error {
	p.calls++
	p.stopped = append([]string{}, s...)
	return p.stopFailure
}

func (p *fakeLifecycleProject) Restart(s []string) error {
	p.calls++
	p.restarted = append([]string{}, s...)
	return p.failure
}

func lifecycleTestEnvironment(t *testing.T) devenv.Environment {
	t.Helper()
	original := devenv.Context
	devenv.Context = "test"
	t.Cleanup(func() { devenv.Context = original })
	return devenv.Environment{BaseDir: t.TempDir()}
}

func TestSuspendResumePreservesIntent(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	p := &fakeLifecycleProject{running: []string{"mysql", "mediawiki"}}
	if err := suspendDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	p.running = nil
	if err := suspendDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	p.exited = []string{"mysql", "mediawiki", "manually-stopped", "one-shot"}
	if err := resumeDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.started, []string{"mediawiki", "mysql"}) {
		t.Fatal(p.started)
	}
	if err := m.UpdateResumeServices([]string{"mysql"}, false); err != nil {
		t.Fatal(err)
	}
	p.started = nil
	if err := resumeDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.started, []string{"mediawiki"}) {
		t.Fatal(p.started)
	}
}

func TestLifecycleEmptyTargetsNeverMeanAll(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	p := &fakeLifecycleProject{}
	if err := suspendDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	p.exited = []string{"one-shot"}
	if err := resumeDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	if err := restartDevelopmentServices(p); err != nil {
		t.Fatal(err)
	}
	if p.calls != 0 {
		t.Fatalf("called compose with empty scope %d", p.calls)
	}
}

func TestLifecycleEnumerationErrorsDoNotMutate(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	failure := errors.New("enumeration failed")
	p := &fakeLifecycleProject{failure: failure}
	for _, err := range []error{suspendDevelopmentServices(m, p), resumeDevelopmentServices(m, p), restartDevelopmentServices(p)} {
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
	}
	if p.calls != 0 {
		t.Fatal("mutated after enumeration failure")
	}
}

func TestSuspendRetryPreservesPartlyStoppedServices(t *testing.T) {
	for _, stillRunning := range [][]string{{"mysql"}, nil} {
		t.Run(fmt.Sprint(stillRunning), func(t *testing.T) {
			m := lifecycleTestEnvironment(t)
			failure := errors.New("stop partly failed")
			p := &fakeLifecycleProject{running: []string{"mediawiki", "mysql"}, stopFailure: failure}
			if err := suspendDevelopmentServices(m, p); !errors.Is(err, failure) {
				t.Fatalf("expected stop failure, got %v", err)
			}
			// The first stop managed to stop some or all targets before failing.
			p.running = stillRunning
			p.stopFailure = nil
			if err := suspendDevelopmentServices(m, p); err != nil {
				t.Fatal(err)
			}
			wanted, _, err := m.ResumeServices()
			if err != nil || !reflect.DeepEqual(wanted, []string{"mediawiki", "mysql"}) {
				t.Fatalf("retry lost the original set: wanted=%v err=%v", wanted, err)
			}
			p.running = nil
			p.exited = []string{"mediawiki", "mysql", "one-shot"}
			if err := resumeDevelopmentServices(m, p); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(p.started, []string{"mediawiki", "mysql"}) {
				t.Fatalf("resumed %v", p.started)
			}
			// A fresh suspension must not merge an old completed snapshot.
			p.running = []string{"mysql"}
			if err := suspendDevelopmentServices(m, p); err != nil {
				t.Fatal(err)
			}
			wanted, _, err = m.ResumeServices()
			if err != nil || !reflect.DeepEqual(wanted, []string{"mysql"}) {
				t.Fatalf("new suspension retained stale services: wanted=%v err=%v", wanted, err)
			}
		})
	}
}

func TestLegacyResumeAndRunningOnlyRestart(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	p := &fakeLifecycleProject{running: []string{"running"}, exited: []string{"existing"}, created: []string{"created"}}
	if err := resumeDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.started, []string{"created", "existing"}) {
		t.Fatal(p.started)
	}
	if err := restartDevelopmentServices(p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.restarted, []string{"running"}) {
		t.Fatal(p.restarted)
	}
}

func TestResumeAfterFailedStopEndsPendingSuspension(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	p := &fakeLifecycleProject{running: []string{"mediawiki", "mysql"}, stopFailure: errors.New("partial stop")}
	if err := suspendDevelopmentServices(m, p); err == nil {
		t.Fatal("expected partial stop failure")
	}
	p.running = []string{"mysql"}
	p.exited = []string{"mediawiki", "one-shot"}
	if err := resumeDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.started, []string{"mediawiki"}) {
		t.Fatalf("resumed %v", p.started)
	}
	p.stopFailure = nil
	if err := suspendDevelopmentServices(m, p); err != nil {
		t.Fatal(err)
	}
	wanted, _, err := m.ResumeServices()
	if err != nil || !reflect.DeepEqual(wanted, []string{"mysql"}) {
		t.Fatalf("resume left an old suspension pending: %v %v", wanted, err)
	}
}
