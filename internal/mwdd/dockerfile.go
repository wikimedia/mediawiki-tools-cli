package mwdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// dockerfileEnvKey returns the .env key used to store a service's custom Dockerfile path.
// For example, service "mediawiki" → "MEDIAWIKI_DOCKERFILE".
func dockerfileEnvKey(service string) string {
	return strings.ToUpper(service) + "_DOCKERFILE"
}

// dockerfileComposeFilePath returns the path of the auto-generated docker compose override
// file that instructs Docker Compose to build from a custom Dockerfile.
func dockerfileComposeFilePath(directory, service string) string {
	return filepath.Join(directory, "custom-dockerfile-"+service+".yml")
}

// dockerfileComposeOverride is the structure marshalled into the auto-generated
// docker compose override YAML file.  Only the fields that are needed for a build
// override are included.
type dockerfileComposeOverride struct {
	Services map[string]dockerfileServiceOverride `yaml:"services"`
}

type dockerfileServiceOverride struct {
	Build dockerfileBuildSpec `yaml:"build"`
	Image string              `yaml:"image"`
}

type dockerfileBuildSpec struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// writeDockerfileComposeFile writes (or overwrites) a docker compose override file that
// builds the service image from the given Dockerfile.  The build context is the directory
// that contains the Dockerfile.  The resulting image is tagged with a predictable name so
// that it can be referenced and rebuilt on demand.
func writeDockerfileComposeFile(directory, service, dockerfilePath string) error {
	absPath, err := filepath.Abs(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to resolve dockerfile path: %w", err)
	}
	override := dockerfileComposeOverride{
		Services: map[string]dockerfileServiceOverride{
			service: {
				Build: dockerfileBuildSpec{
					Context:    filepath.Dir(absPath),
					Dockerfile: filepath.Base(absPath),
				},
				Image: "mwcli-" + service + "-custom:local",
			},
		},
	}
	content, err := yaml.Marshal(override)
	if err != nil {
		return fmt.Errorf("failed to marshal compose override: %w", err)
	}
	return os.WriteFile(dockerfileComposeFilePath(directory, service), content, 0o600)
}

// removeDockerfileComposeFile removes the auto-generated docker compose override file for
// the given service.  It is a no-op if the file does not exist.
func removeDockerfileComposeFile(directory, service string) error {
	path := dockerfileComposeFilePath(directory, service)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

// ensureDockerfileComposeFiles regenerates any auto-generated compose override files for
// services that have a custom Dockerfile stored in the .env file.  This is called from
// EnsureReady so that the files are recreated if they were accidentally deleted.
func (m MWDD) ensureDockerfileComposeFiles() {
	envMap := m.Env().List()
	for key, value := range envMap {
		if strings.HasSuffix(key, "_DOCKERFILE") && value != "" {
			service := strings.ToLower(strings.TrimSuffix(key, "_DOCKERFILE"))
			// Ignore errors here – if the Dockerfile path is stale, the user will see a
			// clear error from Docker Compose when they next try to start the service.
			_ = writeDockerfileComposeFile(m.Directory(), service, value)
		}
	}
}
