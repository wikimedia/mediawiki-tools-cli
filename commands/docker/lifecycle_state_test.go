package docker

import (
	"errors"
	"reflect"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

type fakeLifecycleProject struct {
	running, exited, created, started, stopped, restarted []string
	failure                                               error
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

func (p *fakeLifecycleProject) Start(s []string) error {
	p.calls++
	p.started = append([]string{}, s...)
	return p.failure
}

func (p *fakeLifecycleProject) Stop(s []string) error {
	p.calls++
	p.stopped = append([]string{}, s...)
	return p.failure
}

func (p *fakeLifecycleProject) Restart(s []string) error {
	p.calls++
	p.restarted = append([]string{}, s...)
	return p.failure
}

func lifecycleTestMWDD(t *testing.T) mwdd.MWDD {
	t.Helper()
	original := mwdd.Context
	mwdd.Context = "test"
	t.Cleanup(func() { mwdd.Context = original })
	return mwdd.MWDD{BaseDir: t.TempDir()}
}

func TestSuspendResumePreservesIntent(t *testing.T) {
	m := lifecycleTestMWDD(t)
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
	m := lifecycleTestMWDD(t)
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
	m := lifecycleTestMWDD(t)
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

func TestLegacyResumeAndRunningOnlyRestart(t *testing.T) {
	m := lifecycleTestMWDD(t)
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
