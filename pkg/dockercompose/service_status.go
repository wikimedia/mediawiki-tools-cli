package dockercompose

import (
	"encoding/json"
	"fmt"
	osexec "os/exec"
	"sort"
	"strings"
)

// ServiceStatus describes an existing Compose service, including its lifecycle role.
type ServiceStatus struct {
	Name    string
	State   string
	OneShot bool
}

// ServiceStatuses enumerates existing containers, not all configured services.
// Inspecting Compose's container IDs works with both Compose command variants
// and avoids depending on release-specific `compose ps` JSON output formats.
func (p Project) ServiceStatuses() ([]ServiceStatus, error) {
	stdout, stderr, err := p.Command([]string{"ps", "--all", "--quiet"}).RunAndCollect()
	if err != nil {
		return nil, fmt.Errorf("list service containers: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	ids := strings.Fields(stdout.String())
	if len(ids) == 0 {
		return nil, nil
	}
	output, inspectStderr, err := (Command{Cmd: osexec.Command("docker", append([]string{"container", "inspect", "--"}, ids...)...)}).RunAndCollect()
	if err != nil {
		return nil, fmt.Errorf("inspect service containers: %w: %s", err, strings.TrimSpace(inspectStderr.String()))
	}
	var containers []struct {
		Config struct{ Labels map[string]string }
		State  struct{ Status string }
	}
	if err := json.Unmarshal(output.Bytes(), &containers); err != nil {
		return nil, fmt.Errorf("parse container statuses: %w", err)
	}
	config, err := p.ConfigWithError()
	if err != nil {
		return nil, err
	}
	// A scaled service is running if any of its containers is running.
	services := make(map[string]ServiceStatus)
	for _, container := range containers {
		name := container.Config.Labels["com.docker.compose.service"]
		if name == "" {
			return nil, fmt.Errorf("container status has no Compose service name")
		}
		if _, configured := config.Services[name]; !configured {
			continue
		}
		old, exists := services[name]
		if exists && old.State == "running" {
			continue
		}
		services[name] = ServiceStatus{
			Name: name, State: container.State.Status,
			OneShot: config.Services[name].Labels["org.wikimedia.cli.lifecycle"] == "oneshot" || container.Config.Labels["org.wikimedia.cli.lifecycle"] == "oneshot",
		}
	}
	result := make([]ServiceStatus, 0, len(services))
	for _, service := range services {
		result = append(result, service)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
