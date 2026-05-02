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
	Image       string   `yaml:"image"`
	Entrypoint  string   `yaml:"entrypoint"`
	Volumes     []string `yaml:"volumes"`
	Environment []string `yaml:"environment"`
	DependsOn   []string `yaml:"depends_on"`
	DNS         []string `yaml:"dns"`
	Networks    []string `yaml:"networks"`
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

func (f File) Exists() bool {
	if _, err := os.Stat(f.String()); os.IsNotExist(err) {
		return false
	}
	return true
}

func (f File) ExistsOrExit() {
	if !f.Exists() {
		fmt.Println("docker compose file " + f.String() + " does not exist")
		os.Exit(1)
	}
}

func (f File) Contents() Contents {
	yamlFile, err := os.ReadFile(f.String())
	if err != nil {
		panic(err)
	}
	var contents Contents

	err = yaml.Unmarshal(yamlFile, &contents)
	if err != nil {
		panic(err)
	}

	return contents
}

func (c Contents) ServiceNames() []string {
	var serviceNames []string
	for serviceName := range c.Services {
		serviceNames = append(serviceNames, serviceName)
	}
	return serviceNames
}

func (c Contents) VolumeNames() []string {
	var volumeNames []string
	for volumeName := range c.Volumes {
		volumeNames = append(volumeNames, volumeName)
	}
	return volumeNames
}
