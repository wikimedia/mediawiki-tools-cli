package dockercompose

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Version  string                   `yaml:"version"`
	Services map[string]ServiceConfig `json:"services"`
	Volumes  map[string]interface{}   `json:"volumes"`
}

type ServiceConfig struct {
	Image string `yaml:"image"`
}

// Config returns the docker compose config for the project, fully rendered with env vars.
func (p Project) Config() (Config, error) {
	stdOut, stdErr, err := p.Command([]string{"config"}).RunAndCollect()
	if err != nil {
		return Config{}, fmt.Errorf("run docker compose config: %w", err)
	}
	if stdErr.String() != "" {
		return Config{}, fmt.Errorf("docker compose config: %s", stdErr.String())
	}

	var config Config

	err = yaml.Unmarshal(stdOut.Bytes(), &config)
	if err != nil {
		return Config{}, fmt.Errorf("parse docker compose config: %w", err)
	}

	return config, nil
}
