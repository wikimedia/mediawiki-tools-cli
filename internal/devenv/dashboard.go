package devenv

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/cli/cli/compose/template"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gopkg.in/yaml.v2"
)

// DashboardURL returns the configured local dashboard URL without starting Docker.
func (m Environment) DashboardURL() (string, error) {
	port := os.Getenv("PORT")
	if port == "" {
		values, err := m.Env().Read()
		if err != nil {
			return "", fmt.Errorf("read development context %q: %w", Context, err)
		}
		port = values["PORT"]
	}
	if port == "" {
		return "", fmt.Errorf("no dashboard port configured for development context %q", Context)
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return "", fmt.Errorf("invalid dashboard port %q for development context %q", port, Context)
	}
	return dashboardURL("localhost", uint16(portNumber)), nil
}

type GatewayState string

const (
	GatewayMissing          GatewayState = "not created"
	GatewayStopped          GatewayState = "stopped"
	GatewayStarting         GatewayState = "starting"
	GatewayRunning          GatewayState = "running"
	GatewayDashboardStopped GatewayState = "running, dashboard stopped"
)

type GatewayStatus struct {
	State         GatewayState
	URL           string
	HasContainers bool
}

// GatewayStatus inspects the selected Compose project without loading or creating its files.
func (m Environment) GatewayStatus(ctx context.Context) (GatewayStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	engine, err := docker.NewClientFromEnv()
	if err != nil {
		return GatewayStatus{}, err
	}
	defer engine.Close()
	filters := client.Filters{}.Add("label", "com.docker.compose.project="+strings.ToLower(m.DockerCompose().Name))
	list, err := engine.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: filters})
	if err != nil {
		return GatewayStatus{}, err
	}
	return composeGatewayStatus(list.Items), nil
}

func composeGatewayStatus(containers []container.Summary) GatewayStatus {
	status := GatewayStatus{State: GatewayMissing, HasContainers: len(containers) > 0}
	var dashboardState container.ContainerState
	for _, item := range containers {
		if number := item.Labels["com.docker.compose.container-number"]; number != "" && number != "1" {
			continue
		}
		switch item.Labels["com.docker.compose.service"] {
		case "dashboard":
			dashboardState = item.State
		case "traefik":
			status.State = GatewayStopped
			if item.State == "running" || item.State == "restarting" {
				status.State = GatewayStarting
			}
			if item.State != "running" {
				continue
			}
			for _, port := range item.Ports {
				if port.PublicPort == 0 || port.Type != "tcp" {
					continue
				}
				host := "localhost"
				if port.IP.IsValid() && !port.IP.IsUnspecified() && !port.IP.IsLoopback() {
					host = port.IP.String()
				}
				status.URL = dashboardURL(host, port.PublicPort)
				break
			}
		}
	}
	if status.URL != "" {
		switch dashboardState {
		case "running":
			status.State = GatewayRunning
		case "created", "restarting":
			status.State = GatewayStarting
		default:
			status.State = GatewayDashboardStopped
		}
	}
	return status
}

func dashboardURL(host string, port uint16) string {
	return "http://" + net.JoinHostPort(host, strconv.Itoa(int(port))) + "/proxy/dashboard/"
}

// RunningDashboardURL supplies an optional link for commands that may leave the gateway stopped.
func (m Environment) RunningDashboardURL() string {
	status, err := m.GatewayStatus(context.Background())
	if err == nil && status.State == GatewayRunning {
		return status.URL
	}
	return ""
}

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
	environment, err := m.Env().Read()
	if err != nil {
		return err
	}
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
