package devenv

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

// ServiceIntent records desired lifecycle state rather than Docker exit status.
type ServiceIntent struct {
	Name    string
	Desired string
	OneShot bool
}

// ServiceIntents returns all recorded created services, including stopped ones.
func (m Environment) ServiceIntents() ([]ServiceIntent, error) {
	state, _, err := m.readResumeState()
	if err != nil {
		return nil, err
	}
	running := serviceNameSet(state.Services)
	jobs := serviceNameSet(state.OneShot)
	all := serviceNameSet(append(append([]string{}, state.Services...), state.Stopped...))
	result := make([]ServiceIntent, 0, len(all))
	for _, name := range sortedServiceNames(all) {
		desired := "stopped"
		if running[name] && !jobs[name] {
			desired = "running"
		}
		result = append(result, ServiceIntent{Name: name, Desired: desired, OneShot: jobs[name]})
	}
	return result, nil
}

// EnsureServiceIntent migrates a legacy snapshot, or adopts running containers
// when no snapshot exists. Exited/created containers are never guessed running.
func (m Environment) EnsureServiceIntent(inventory []dockercompose.ServiceStatus) error {
	adoptedStopped := false
	err := m.mutateResumeState(func(state *resumeState, exists bool) error {
		running, stopped, jobs := serviceNameSet(state.Services), serviceNameSet(state.Stopped), serviceNameSet(state.OneShot)
		for _, service := range inventory {
			if service.OneShot {
				jobs[service.Name] = true
				delete(running, service.Name)
				stopped[service.Name] = true
				continue
			}
			if state.Version == 2 {
				continue
			}
			if running[service.Name] {
				continue
			}
			if !exists && service.State == "running" {
				running[service.Name] = true
			} else {
				stopped[service.Name] = true
				adoptedStopped = adoptedStopped || !exists
			}
		}
		state.Version = 2
		state.Services, state.Stopped, state.OneShot = sortedServiceNames(running), sortedServiceNames(stopped), sortedServiceNames(jobs)
		return nil
	})
	if err == nil && adoptedStopped {
		logrus.Warn("No saved service intent exists for this context. Existing stopped containers remain stopped; use 'mw dev <service> start' for services you want to resume.")
	}
	return err
}

// ServiceAction describes an explicit change to created-service intent.
type ServiceAction string

const (
	ServiceStarted   ServiceAction = "started"
	ServiceStopped   ServiceAction = "stopped"
	ServiceDestroyed ServiceAction = "destroyed"
)

// ChangeServices initializes intent before Docker changes and records the result
// only after success. Newly started long-running dependencies are included.
func (m Environment) ChangeServices(ctx context.Context, services []string, action ServiceAction, operation func() error) error {
	if len(services) == 0 {
		return fmt.Errorf("no services selected for lifecycle action")
	}
	if action != ServiceStarted && action != ServiceStopped && action != ServiceDestroyed {
		return fmt.Errorf("unknown service action %q", action)
	}
	return m.WithServiceLifecycle(ctx, func() error {
		m.EnsureReady()
		before, err := m.DockerCompose().ServiceStatuses()
		if err != nil {
			return err
		}
		if err := m.EnsureServiceIntent(before); err != nil {
			return err
		}
		if err := operation(); err != nil {
			return err
		}
		if action == ServiceDestroyed {
			return m.recordServiceAction(services, action, before, nil)
		}
		after, err := m.DockerCompose().ServiceStatuses()
		if err != nil {
			return fmt.Errorf("Docker action completed but service intent could not be refreshed: %w", err)
		}
		return m.recordServiceAction(services, action, before, after)
	})
}

func (m Environment) recordServiceAction(services []string, action ServiceAction, before, after []dockercompose.ServiceStatus) error {
	return m.mutateResumeState(func(state *resumeState, exists bool) error {
		state.Version = 2
		running, stopped, jobs := serviceNameSet(state.Services), serviceNameSet(state.Stopped), serviceNameSet(state.OneShot)
		selected := serviceNameSet(services)
		if action == ServiceDestroyed {
			for name := range selected {
				delete(running, name)
				delete(stopped, name)
				delete(jobs, name)
			}
		} else {
			wasRunning := make(map[string]bool)
			for _, service := range before {
				wasRunning[service.Name] = service.State == "running"
			}
			for _, service := range after {
				dependencyStarted := action == ServiceStarted && service.State == "running" && !wasRunning[service.Name]
				if !selected[service.Name] && !dependencyStarted {
					continue
				}
				if service.OneShot {
					jobs[service.Name] = true
				} else {
					delete(jobs, service.Name)
				}
				if action == ServiceStarted && !service.OneShot {
					running[service.Name] = true
					delete(stopped, service.Name)
				} else {
					stopped[service.Name] = true
					delete(running, service.Name)
				}
			}
		}
		state.Services, state.Stopped, state.OneShot = sortedServiceNames(running), sortedServiceNames(stopped), sortedServiceNames(jobs)
		return nil
	})
}
