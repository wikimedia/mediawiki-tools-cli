package docker

import (
	"fmt"
	"sort"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

type lifecycleProject interface {
	ServicesWithStatus(string) ([]string, error)
	Start([]string) error
	Stop([]string) error
	Restart([]string) error
}

func suspendDevelopmentServices(m mwdd.MWDD, p lifecycleProject) error {
	running, err := p.ServicesWithStatus("running")
	if err != nil {
		return fmt.Errorf("list running containers: %w", err)
	}
	if len(running) == 0 {
		// Do not replace a useful suspended snapshot on a repeated stop.
		if _, exists, err := m.ResumeServices(); err != nil {
			return err
		} else if exists {
			return nil
		}
	}
	// Persist before stop, so a partial stop failure still leaves a resume target.
	if err := m.SaveResumeServices(running); err != nil {
		return err
	}
	if len(running) == 0 {
		return nil
	}
	return lifecycleError("stop", p.Stop(running))
}

func resumeDevelopmentServices(m mwdd.MWDD, p lifecycleProject) error {
	wanted, hasSnapshot, err := m.ResumeServices()
	if err != nil {
		return err
	}
	// Compose calls this state exited, not stopped. Include created containers for
	// legacy contexts that have never recorded a suspend set.
	stopped, err := p.ServicesWithStatus("exited")
	if err != nil {
		return fmt.Errorf("list exited containers: %w", err)
	}
	created, err := p.ServicesWithStatus("created")
	if err != nil {
		return fmt.Errorf("list created containers: %w", err)
	}
	available := append(stopped, created...)
	selected := map[string]bool{}
	intent := map[string]bool{}
	for _, name := range wanted {
		intent[name] = true
	}
	for _, name := range available {
		if !hasSnapshot || intent[name] {
			selected[name] = true
		}
	}
	targets := make([]string, 0, len(selected))
	for name := range selected {
		targets = append(targets, name)
	}
	sort.Strings(targets)
	// Empty Compose arguments mean ALL services, so never call Start with none.
	if len(targets) == 0 {
		return nil
	}
	return lifecycleError("start", p.Start(targets))
}

func restartDevelopmentServices(p lifecycleProject) error {
	running, err := p.ServicesWithStatus("running")
	if err != nil {
		return fmt.Errorf("list running containers: %w", err)
	}
	if len(running) == 0 {
		return nil
	}
	return lifecycleError("restart", p.Restart(running))
}
