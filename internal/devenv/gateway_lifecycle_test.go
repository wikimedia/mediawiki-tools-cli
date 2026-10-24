package devenv

import (
	"errors"
	"reflect"
	"testing"
)

type fakeGatewayServices struct {
	running, restarting, paused, stopped []string
	listErr, stopErr                     error
}

func (p *fakeGatewayServices) ServicesWithStatus(status string) ([]string, error) {
	switch status {
	case "restarting":
		return p.restarting, p.listErr
	case "paused":
		return p.paused, p.listErr
	default:
		return p.running, p.listErr
	}
}

func (p *fakeGatewayServices) Stop(services []string) error {
	p.stopped = append(p.stopped, services...)
	return p.stopErr
}

func TestGatewayCleanupPreservesResumeDependencies(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		running, saved, wantSaved, wantStopped []string
	}{
		{name: "suspended Cloudflare", saved: []string{"cloudflared", "gateway-auth", "traefik"}, wantSaved: []string{"cloudflared", "gateway-auth", "traefik"}},
		{name: "suspended Holesail", saved: []string{"gateway-auth", "holesail", "traefik"}, wantSaved: []string{"gateway-auth", "holesail", "traefik"}},
		{name: "active Cloudflare", running: []string{"cloudflared", "gateway-auth"}, saved: []string{"cloudflared", "gateway-auth"}, wantSaved: []string{"cloudflared", "gateway-auth"}},
		{name: "active Holesail", running: []string{"gateway-auth", "holesail"}, saved: []string{"gateway-auth", "holesail"}, wantSaved: []string{"gateway-auth", "holesail"}},
		{name: "no remaining intent", running: []string{"gateway-auth"}, saved: []string{"gateway-auth", "mysql"}, wantSaved: []string{"mysql"}, wantStopped: []string{"gateway-auth"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := Context
			Context = "test"
			t.Cleanup(func() { Context = previous })
			m := Environment{BaseDir: t.TempDir()}
			if err := m.SaveResumeServices(tc.saved); err != nil {
				t.Fatal(err)
			}
			project := &fakeGatewayServices{running: tc.running}
			if err := m.stopGatewayAuthIfNoTunnel(project); err != nil {
				t.Fatal(err)
			}
			saved, _, err := m.ResumeServices()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved, tc.wantSaved) {
				t.Errorf("saved services = %v, want %v", saved, tc.wantSaved)
			}
			if !reflect.DeepEqual(project.stopped, tc.wantStopped) {
				t.Errorf("stopped services = %v, want %v", project.stopped, tc.wantStopped)
			}
		})
	}
}

func TestGatewayCleanupDoesNotForgetAuthOnStopFailure(t *testing.T) {
	previous := Context
	Context = "test"
	t.Cleanup(func() { Context = previous })
	m := Environment{BaseDir: t.TempDir()}
	if err := m.SaveResumeServices([]string{"gateway-auth"}); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("stop failed")
	project := &fakeGatewayServices{running: []string{"gateway-auth"}, stopErr: failure}
	if err := m.stopGatewayAuthIfNoTunnel(project); !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	saved, _, err := m.ResumeServices()
	if err != nil || !reflect.DeepEqual(saved, []string{"gateway-auth"}) {
		t.Fatalf("resume intent lost: %v, %v", saved, err)
	}
}

func TestGatewayCleanupKeepsAuthForRecoveringProvider(t *testing.T) {
	for _, status := range []string{"restarting", "paused"} {
		t.Run(status, func(t *testing.T) {
			project := &fakeGatewayServices{running: []string{"gateway-auth"}}
			if status == "restarting" {
				project.restarting = []string{"cloudflared"}
			} else {
				project.paused = []string{"holesail"}
			}
			m := Environment{BaseDir: t.TempDir()}
			if err := m.stopGatewayAuthIfNoTunnel(project); err != nil {
				t.Fatal(err)
			}
			if len(project.stopped) != 0 {
				t.Fatalf("stopped authentication for %s provider", status)
			}
		})
	}
}
