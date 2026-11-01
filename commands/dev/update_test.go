package dev

import (
	"errors"
	"reflect"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

type fakeUpdateProject struct {
	inventory       []dockercompose.ServiceStatus
	pulled, started []string
	options         dockercompose.UpOptions
	failure         error
}

func (p *fakeUpdateProject) ServiceStatuses() ([]dockercompose.ServiceStatus, error) {
	return p.inventory, nil
}
func (p *fakeUpdateProject) Pull(names []string) error { p.pulled = names; return p.failure }
func (p *fakeUpdateProject) Up(names []string, opts dockercompose.UpOptions) error {
	p.started = names
	p.options = opts
	return nil
}

func TestUpdatePreservesStoppedServicesAndExcludesJobsAndTunnels(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	p := &fakeUpdateProject{inventory: []dockercompose.ServiceStatus{{Name: "mysql", State: "running"}, {Name: "redis", State: "exited"}, {Name: "setup", State: "running", OneShot: true}, {Name: "gateway-auth", State: "running"}, {Name: "holesail", State: "running"}}}
	if err := updateDevelopmentServices(m, p, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.pulled, []string{"mysql", "redis"}) || !reflect.DeepEqual(p.started, []string{"mysql"}) {
		t.Fatalf("pulled %v started %v", p.pulled, p.started)
	}
	if !p.options.NoDeps || !p.options.Detached || !p.options.ForceRecreate {
		t.Fatal(p.options)
	}
}

func TestUpdateNeverStartsEmptyOrFullyStoppedEnvironments(t *testing.T) {
	for _, inventory := range [][]dockercompose.ServiceStatus{nil, {{Name: "redis", State: "exited"}}, {{Name: "setup", State: "exited", OneShot: true}}} {
		m := lifecycleTestEnvironment(t)
		p := &fakeUpdateProject{inventory: inventory}
		if err := updateDevelopmentServices(m, p, false); err != nil {
			t.Fatal(err)
		}
		if len(p.started) != 0 {
			t.Fatalf("started empty/stopped selection: %v", p.started)
		}
	}
}

func TestUpdateDoesNotRecreateAfterFailedPull(t *testing.T) {
	m := lifecycleTestEnvironment(t)
	failure := errors.New("pull failed")
	p := &fakeUpdateProject{inventory: []dockercompose.ServiceStatus{{Name: "mysql", State: "running"}}, failure: failure}
	if err := updateDevelopmentServices(m, p, false); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(p.started) != 0 {
		t.Fatal(p.started)
	}
}
