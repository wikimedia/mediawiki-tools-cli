package dev

import (
	"fmt"
	"sort"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

type lifecycleProject interface {
	ServiceStatuses() ([]dockercompose.ServiceStatus, error)
	StartContainers([]string) error
	Stop([]string) error
	Restart([]string) error
}

func suspendDevelopmentServices(m devenv.Environment, p lifecycleProject) error {
	inventory, err := p.ServiceStatuses()
	if err != nil {
		return err
	}
	if err := m.EnsureServiceIntent(inventory); err != nil {
		return err
	}
	var running []string
	for _, service := range inventory {
		if service.State == "running" {
			running = append(running, service.Name)
		}
	}
	sort.Strings(running)
	// Persist before stop, so a partial stop failure still leaves a resume target.
	if err := m.BeginSuspendServices(running); err != nil {
		return err
	}
	if len(running) > 0 {
		if err := p.Stop(running); err != nil {
			return lifecycleError("stop", err)
		}
	}
	return m.CompleteSuspendServices()
}

func resumeDevelopmentServices(m devenv.Environment, p lifecycleProject) error {
	inventory, err := p.ServiceStatuses()
	if err != nil {
		return err
	}
	if err := m.EnsureServiceIntent(inventory); err != nil {
		return err
	}
	wanted, _, err := m.ResumeServices()
	if err != nil {
		return err
	}
	intent := make(map[string]bool, len(wanted))
	for _, name := range wanted {
		intent[name] = true
	}
	selected := map[string]bool{}
	for _, service := range inventory {
		if !service.OneShot && intent[service.Name] && (service.State == "exited" || service.State == "created") {
			selected[service.Name] = true
		}
	}
	targets := make([]string, 0, len(selected))
	for name := range selected {
		targets = append(targets, name)
	}
	sort.Strings(targets)
	// Empty Compose arguments mean ALL services, so never start with none.
	if len(targets) == 0 {
		return m.CompleteSuspendServices()
	}
	if err := p.StartContainers(targets); err != nil {
		return lifecycleError("start", err)
	}
	return m.CompleteSuspendServices()
}

func restartDevelopmentServices(p lifecycleProject) error {
	inventory, err := p.ServiceStatuses()
	if err != nil {
		return fmt.Errorf("list running containers: %w", err)
	}
	var running []string
	for _, service := range inventory {
		if service.State == "running" && !service.OneShot {
			running = append(running, service.Name)
		}
	}
	sort.Strings(running)
	if len(running) == 0 {
		return nil
	}
	return lifecycleError("restart", p.Restart(running))
}
