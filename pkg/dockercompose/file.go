package dockercompose

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type File string

type Contents struct {
	Version  string                     `yaml:"version"`
	Services map[string]ServiceContents `json:"services"`
	Volumes  map[string]interface{}     `json:"volumes"`
}

type ServiceContents struct {
	Profiles    []string `yaml:"profiles"`
	Image       string   `yaml:"image"`
	Entrypoint  string   `yaml:"entrypoint"`
	Volumes     []string `yaml:"volumes"`
	Environment []string `yaml:"environment"`
	// Compose supports both a list and a mapping with health-check conditions.
	DependsOn interface{} `yaml:"depends_on"`
	DNS       []string    `yaml:"dns"`
	Networks  []string    `yaml:"networks"`
}

func (p Project) File(name string) File {
	preferred := filepath.Join(p.Directory, "compose", name, "compose.yml")
	if _, err := os.Stat(preferred); err == nil {
		return File(preferred)
	}

	// Legacy fallback.
	return File(filepath.Join(p.Directory, name+".yml"))
}

func (f File) String() string {
	return string(f)
}

func (f File) RequireExists() error {
	if _, err := os.Stat(f.String()); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("docker compose file %s does not exist", f.String())
		}
		return fmt.Errorf("check docker compose file %s: %w", f.String(), err)
	}
	return nil
}

func (f File) Contents() (Contents, error) {
	yamlFile, err := os.ReadFile(f.String())
	if err != nil {
		return Contents{}, fmt.Errorf("read docker compose file %s: %w", f.String(), err)
	}
	var contents Contents

	err = yaml.Unmarshal(yamlFile, &contents)
	if err != nil {
		return Contents{}, fmt.Errorf("parse docker compose file %s: %w", f.String(), err)
	}

	return contents, nil
}

func (c Contents) ServiceNames() []string {
	var serviceNames []string
	for serviceName := range c.Services {
		serviceNames = append(serviceNames, serviceName)
	}
	return serviceNames
}

// StartServiceNames keeps optional services opt-in, matching Compose's profile
// behaviour, while allowing the explicitly requested service to use a profile.
func (c Contents) StartServiceNames(primary string) []string {
	var names []string
	for name, service := range c.Services {
		if len(service.Profiles) == 0 || name == primary {
			names = append(names, name)
		}
	}
	return names
}

func (c Contents) VolumeNames() []string {
	var volumeNames []string
	for volumeName := range c.Volumes {
		volumeNames = append(volumeNames, volumeName)
	}
	return volumeNames
}
