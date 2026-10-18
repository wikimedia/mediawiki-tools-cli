package dockercompose

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Version  string                   `yaml:"version"`
	Services map[string]ServiceConfig `json:"services"`
	Volumes  map[string]interface{}   `json:"volumes"`
}

type ServiceConfig struct {
	Image  string            `yaml:"image"`
	Labels map[string]string `yaml:"labels"`
}

// Config returns the docker compose config for the project, fully rendered with env vars.
func (p Project) Config() Config {
	config, err := p.ConfigWithError()
	if err != nil {
		panic(err)
	}
	return config
}

// ConfigWithError reads rendered Compose metadata without terminating the caller.
func (p Project) ConfigWithError() (Config, error) {
	stdOut, stdErr, err := p.Command([]string{"config"}).RunAndCollect()
	if err != nil {
		return Config{}, fmt.Errorf("read compose config: %w: %s", err, strings.TrimSpace(stdErr.String()))
	}
	var config Config
	if err := yaml.Unmarshal(stdOut.Bytes(), &config); err != nil {
		return Config{}, fmt.Errorf("parse compose config: %w", err)
	}
	return config, nil
}
