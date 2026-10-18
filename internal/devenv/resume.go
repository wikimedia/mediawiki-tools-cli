package devenv

import (
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
	if state.Version != 1 {
		return resumeState{}, true, fmt.Errorf("unsupported resume state version %d", state.Version)
	}
	for _, name := range state.Services {
		if !resumeServiceName.MatchString(name) {
			return resumeState{}, true, fmt.Errorf("invalid service name in resume state %q", name)
		}
	}
	return state, true, nil
}

func (m Environment) SaveResumeServices(services []string) error {
	return m.saveResumeState(resumeState{Version: 1, Services: services})
}

// BeginSuspendServices preserves the full target set while retrying an
// incomplete stop. A fresh stop records only the currently running services.
func (m Environment) BeginSuspendServices(running []string) error {
	state, exists, err := m.readResumeState()
	if err != nil {
		return err
	}
	if state.SuspendPending {
		state.Services = append(state.Services, running...)
	} else if len(running) == 0 && exists {
		return nil
	} else {
		state.Services = running
	}
	state.Version = 1
	state.SuspendPending = true
	return m.saveResumeState(state)
}

// CompleteSuspendServices ends retry tracking after a successful stop or resume.
func (m Environment) CompleteSuspendServices() error {
	state, exists, err := m.readResumeState()
	if err != nil {
		return err
	}
	if !exists || !state.SuspendPending {
		return nil
	}
	state.SuspendPending = false
	return m.saveResumeState(state)
}

func (m Environment) saveResumeState(state resumeState) error {
	unique := map[string]bool{}
	for _, name := range state.Services {
		if !resumeServiceName.MatchString(name) {
			return fmt.Errorf("invalid resume service name %q", name)
		}
		unique[name] = true
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	state.Services = names
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
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(path, m.resumeStatePath())
}

// UpdateResumeServices records explicit CLI service actions only when a snapshot
// already exists. Direct Docker/Compose actions are intentionally not inferred.
func (m Environment) UpdateResumeServices(services []string, add bool) error {
	state, exists, err := m.readResumeState()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	wanted := map[string]bool{}
	for _, name := range state.Services {
		wanted[name] = true
	}
	for _, name := range services {
		if add {
			wanted[name] = true
		} else {
			delete(wanted, name)
		}
	}
	state.Services = nil
	for name := range wanted {
		state.Services = append(state.Services, name)
	}
	return m.saveResumeState(state)
}

func (m Environment) MergeRunningResumeServices() error {
	_, exists, err := m.ResumeServices()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	running, err := m.DockerCompose().ServicesWithStatus("running")
	if err != nil {
		return err
	}
	return m.UpdateResumeServices(running, true)
}

func (m Environment) ClearResumeServices() error {
	err := os.Remove(m.resumeStatePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
