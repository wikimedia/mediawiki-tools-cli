package dockercompose

import (
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

// Config returns the docker-compose config for the project, fully rendered with env vars.
func (p Project) Config() Config {
	stdOut, stdErr, err := p.Command([]string{"config"}).RunAndCollect()
	if err != nil {
		panic(err)
	}
	if stdErr.String() != "" {
		panic(stdErr)
	}

	var config Config

	err = yaml.Unmarshal(stdOut.Bytes(), &config)
	if err != nil {
		panic(err)
	}

	return config
}
