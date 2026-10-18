package devenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/docker/cli/cli/compose/template"
	"gopkg.in/yaml.v2"
)

// dashboardService contains display metadata only, never the full environment.
type dashboardService struct {
	Name   string `json:"name"`
	HasUI  bool   `json:"has_ui"`
	UIHost string `json:"ui_host,omitempty"`
}

var dashboardHostname = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (m Environment) refreshDashboardServices() error {
	files, err := m.DockerCompose().ComposeFiles()
	if err != nil {
		return err
	}
	environment := m.Env().List()
	lookup := func(key string) (string, bool) {
		if value, ok := os.LookupEnv(key); ok {
			return value, true
		}
		value, ok := environment[key]
		return value, ok
	}
	services := map[string]dashboardService{}
	for _, file := range files {
		name := filepath.Base(file)
		if !strings.HasPrefix(name, "custom") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.Directory(), file))
		if err != nil {
			return err
		}
		var config struct {
			Services map[string]struct {
				Environment interface{} `yaml:"environment"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("read dashboard metadata from %s: %w", file, err)
		}
		for name, service := range config.Services {
			value, defined := dashboardVirtualHost(service.Environment, lookup)
			entry, exists := services[name]
			if !exists {
				entry = dashboardService{Name: name}
			}
			if defined {
				host, err := template.Substitute(value, lookup)
				if err != nil {
					return fmt.Errorf("dashboard host for %s: %w", name, err)
				}
				host = strings.TrimSpace(strings.Split(host, ",")[0])
				entry.HasUI = dashboardHostname.MatchString(host)
				entry.UIHost = ""
				if entry.HasUI {
					entry.UIHost = host
				}
			}
			services[name] = entry
		}
	}
	result := make([]dashboardService, 0, len(services))
	for _, service := range services {
		result = append(result, service)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	// The directory is mounted into the dashboard, so atomic rename is visible
	// without adding another file bind mount or recreating the dashboard.
	temp, err := os.CreateTemp(m.Directory(), ".dashboard-services-*")
	if err != nil {
		return err
	}
	path := temp.Name()
	defer os.Remove(path)
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return err
	}
	return os.Rename(path, filepath.Join(m.Directory(), "dashboard-services.json"))
}

func dashboardVirtualHost(environment interface{}, lookup func(string) (string, bool)) (string, bool) {
	switch values := environment.(type) {
	case []interface{}:
		for _, raw := range values {
			value, ok := raw.(string)
			if !ok {
				continue
			}
			key, host, hasValue := strings.Cut(value, "=")
			if key == "VIRTUAL_HOST" {
				if !hasValue {
					host, _ = lookup(key)
				}
				return host, true
			}
		}
	case map[interface{}]interface{}:
		if value, ok := values["VIRTUAL_HOST"]; ok {
			if value == nil {
				value, _ = lookup("VIRTUAL_HOST")
			}
			return fmt.Sprint(value), true
		}
	}
	return "", false
}
