package devenv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type resumeState struct {
	Version        int      `json:"version"`
	Services       []string `json:"services"`
	Stopped        []string `json:"stopped,omitempty"`
	OneShot        []string `json:"one_shot,omitempty"`
	SuspendPending bool     `json:"suspend_pending,omitempty"`
}

var resumeServiceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (m Environment) resumeStatePath() string {
	return filepath.Join(m.Directory(), "state-suspended-services.json")
}

// ResumeServices distinguishes an absent legacy record from a known empty set.
func (m Environment) ResumeServices() ([]string, bool, error) {
	state, exists, err := m.readResumeState()
	return state.Services, exists, err
}

func (m Environment) readResumeState() (resumeState, bool, error) {
	data, err := os.ReadFile(m.resumeStatePath())
	if os.IsNotExist(err) {
		return resumeState{}, false, nil
	}
	if err != nil {
		return resumeState{}, false, err
	}
	var state resumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return resumeState{}, true, fmt.Errorf("read resume state: %w", err)
	}
	if state.Version != 1 && state.Version != 2 {
		return resumeState{}, true, fmt.Errorf("unsupported resume state version %d", state.Version)
	}
	for _, name := range append(append(append([]string{}, state.Services...), state.Stopped...), state.OneShot...) {
		if !resumeServiceName.MatchString(name) {
			return resumeState{}, true, fmt.Errorf("invalid service name in resume state %q", name)
		}
	}
	return state, true, nil
}

// SaveResumeServices replaces a legacy suspend snapshot. Lifecycle actions use
// EnsureServiceIntent and ChangeServices so stopped-service records are retained.
func (m Environment) SaveResumeServices(services []string) error {
	return m.mutateResumeState(func(state *resumeState, exists bool) error {
		*state = resumeState{Version: 1, Services: services}
		return nil
	})
}

// BeginSuspendServices preserves the full target set while retrying an
// incomplete stop. Version-two records also retain desired services which are
// temporarily not running, such as after a machine restart.
func (m Environment) BeginSuspendServices(running []string) error {
	return m.mutateResumeState(func(state *resumeState, exists bool) error {
		if state.Version == 2 {
			// A machine restart or crash must not erase recorded running intent.
			stopped := serviceNameSet(append(append([]string{}, state.Stopped...), state.OneShot...))
			for _, name := range running {
				if !stopped[name] {
					state.Services = append(state.Services, name)
				}
			}
		} else if state.SuspendPending {
			state.Services = append(state.Services, running...)
		} else if len(running) == 0 && exists {
			return nil
		} else {
			state.Services = running
		}
		if state.Version == 0 {
			state.Version = 1
		}
		state.SuspendPending = true
		return nil
	})
}

// CompleteSuspendServices ends retry tracking without discarding running intent.
func (m Environment) CompleteSuspendServices() error {
	return m.mutateResumeState(func(state *resumeState, exists bool) error {
		if exists {
			state.SuspendPending = false
		}
		return nil
	})
}

// mutateResumeState holds a separate state lock so exported state APIs remain
// safe even when called outside the broader lifecycle-operation lock.
func (m Environment) mutateResumeState(change func(*resumeState, bool) error) error {
	return m.withServiceLock(context.Background(), ".service-state.lock", func() error {
		state, exists, err := m.readResumeState()
		if err != nil {
			return err
		}
		if err := change(&state, exists); err != nil {
			return err
		}
		if state.Version == 0 {
			return nil
		}
		return m.saveResumeState(state)
	})
}

func (m Environment) saveResumeState(state resumeState) error {
	for _, names := range [][]string{state.Services, state.Stopped, state.OneShot} {
		for _, name := range names {
			if !resumeServiceName.MatchString(name) {
				return fmt.Errorf("invalid resume service name %q", name)
			}
		}
	}
	running := serviceNameSet(state.Services)
	stopped := serviceNameSet(state.Stopped)
	for _, name := range state.OneShot {
		delete(running, name)
		stopped[name] = true
	}
	for name := range running {
		delete(stopped, name)
	}
	state.Services = sortedServiceNames(running)
	state.Stopped = sortedServiceNames(stopped)
	state.OneShot = sortedServiceNames(serviceNameSet(state.OneShot))
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.Directory(), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(m.Directory(), ".resume-services-*")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(path, m.resumeStatePath())
}

// UpdateResumeServices changes running/stopped intent after an explicit action.
// Runtime callers initialize the inventory before mutating Docker.
func (m Environment) UpdateResumeServices(services []string, add bool) error {
	return m.mutateResumeState(func(state *resumeState, exists bool) error {
		if !exists {
			state.Version = 2
		}
		running, stopped := serviceNameSet(state.Services), serviceNameSet(state.Stopped)
		for _, name := range services {
			if add {
				running[name] = true
				delete(stopped, name)
			} else {
				delete(running, name)
				if state.Version == 2 {
					stopped[name] = true
				}
			}
		}
		state.Services, state.Stopped = sortedServiceNames(running), sortedServiceNames(stopped)
		return nil
	})
}

// MergeRunningResumeServices adopts the current inventory and records running
// services. New lifecycle commands use ChangeServices to scope explicit changes.
func (m Environment) MergeRunningResumeServices() error {
	inventory, err := m.DockerCompose().ServiceStatuses()
	if err != nil {
		return err
	}
	if err := m.EnsureServiceIntent(inventory); err != nil {
		return err
	}
	var running []string
	for _, service := range inventory {
		if service.State == "running" && !service.OneShot {
			running = append(running, service.Name)
		}
	}
	return m.UpdateResumeServices(running, true)
}

func (m Environment) ClearResumeServices() error {
	return m.withServiceLock(context.Background(), ".service-state.lock", func() error {
		err := os.Remove(m.resumeStatePath())
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
}

func serviceNameSet(names []string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		result[name] = true
	}
	return result
}

func sortedServiceNames(names map[string]bool) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
