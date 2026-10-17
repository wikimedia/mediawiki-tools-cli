package mwdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type resumeState struct {
	Version  int      `json:"version"`
	Services []string `json:"services"`
}

var resumeServiceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (m MWDD) resumeStatePath() string {
	return filepath.Join(m.Directory(), "state-suspended-services.json")
}

// ResumeServices distinguishes an absent legacy record from a known empty set.
func (m MWDD) ResumeServices() ([]string, bool, error) {
	data, err := os.ReadFile(m.resumeStatePath())
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var state resumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, true, fmt.Errorf("read resume state: %w", err)
	}
	if state.Version != 1 {
		return nil, true, fmt.Errorf("unsupported resume state version %d", state.Version)
	}
	for _, name := range state.Services {
		if !resumeServiceName.MatchString(name) {
			return nil, true, fmt.Errorf("invalid service name in resume state %q", name)
		}
	}
	return state.Services, true, nil
}

func (m MWDD) SaveResumeServices(services []string) error {
	unique := map[string]bool{}
	for _, name := range services {
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
	data, err := json.MarshalIndent(resumeState{Version: 1, Services: names}, "", "  ")
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
func (m MWDD) UpdateResumeServices(services []string, add bool) error {
	names, exists, err := m.ResumeServices()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	for _, name := range services {
		if add {
			wanted[name] = true
		} else {
			delete(wanted, name)
		}
	}
	names = nil
	for name := range wanted {
		names = append(names, name)
	}
	return m.SaveResumeServices(names)
}

func (m MWDD) MergeRunningResumeServices() error {
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

func (m MWDD) ClearResumeServices() error {
	err := os.Remove(m.resumeStatePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
