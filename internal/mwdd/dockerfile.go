package mwdd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
	"gitlab.wikimedia.org/repos/releng/cli/mount"
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

// extractDefaultImageRe matches "${VAR:-default}" in docker compose image fields.
// Compiled once at package init to avoid per-call overhead.
var extractDefaultImageRe = regexp.MustCompile(`\$\{[^}:-]+:-([^}]+)\}`)
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

// defaultDockerfilePath returns the conventional location for a managed Dockerfile
// inside the mwdd working directory (next to custom compose files).
// For example, service "mediawiki" → "<directory>/Dockerfile.mediawiki".
func defaultDockerfilePath(directory, service string) string {
	return filepath.Join(directory, "Dockerfile."+service)
}

// createStarterDockerfileIfNotExists creates a starter Dockerfile at path for the
// given service.  If the file already exists this is a no-op so that user edits are
// never overwritten.
func createStarterDockerfileIfNotExists(path, service string) error {
	if _, err := os.Stat(path); err == nil {
		// File already exists; leave it alone.
		return nil
	}
	return os.WriteFile(path, []byte(starterDockerfileContent(service)), 0o644)
}

// starterDockerfileContent returns the content for a newly created starter Dockerfile.
func starterDockerfileContent(service string) string {
	baseImage := defaultImageForService(service)
	fromLine := "FROM " + baseImage
	if baseImage == "" {
		fromLine = "# TODO: set the base image below, e.g.:\n# FROM <base-image>"
	}
	return fmt.Sprintf(`# Custom Dockerfile for the %s service.
# Edit this file to customise the image (e.g. add packages), then run:
#   mw docker %s create --force-recreate

%s

# Example: uncomment the lines below to install extra packages.
# # 1. Switch to root user to install packages
# USER root
#
# # 2. Remove the Sury PHP repository completely due to expired keys
# RUN rm -f /etc/apt/sources.list.d/php.list \
#     && rm -f /etc/apt/sources.list.d/*sury*.list
#
# # 3. Install extra packages (update the list as needed)
# RUN apt-get update \
#     && apt-get install -y --no-install-recommends \
#         php-wikidiff2 \
#         djvulibre-bin \
#         ffmpeg \
#         netpbm \
#     && rm -rf /var/lib/apt/lists/*
#
# # 4. Switch back to the default non-root user
# USER www-data
`, service, service, fromLine)
}

// defaultImageForService reads the embedded compose file for the service and returns
// the default image name.  Returns an empty string if the information cannot be found.
func defaultImageForService(service string) string {
	composeFilePath := "dev/compose/" + service + "/compose.yml"
	data, err := mount.DevContent.ReadFile(composeFilePath)
	if err != nil {
		return ""
	}

	var contents struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &contents); err != nil {
		return ""
	}

	svc, ok := contents.Services[service]
	if !ok {
		return ""
	}

	image := svc.Image
	// Extract the default from "${VAR:-default}" syntax used in compose files.
	if matches := extractDefaultImageRe.FindStringSubmatch(image); len(matches) > 1 {
		return matches[1]
	}
	return image
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
