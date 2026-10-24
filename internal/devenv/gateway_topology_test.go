package devenv

import (
	"slices"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/mount"
	"gopkg.in/yaml.v2"
)

func TestGatewayIngressIsSeparateFromLocalProxy(t *testing.T) {
	type service struct {
		Ports, Networks, Command, Environment []string
		Restart                               string
	}
	read := func(name string) map[string]service {
		t.Helper()
		data, err := mount.DevContent.ReadFile("dev/compose/" + name + "/compose.yml")
		if err != nil {
			t.Fatal(err)
		}
		var config struct{ Services map[string]service }
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatal(err)
		}
		return config.Services
	}
	proxy := read("traefik")
	if proxy["gateway-auth"].Restart != "no" {
		t.Fatal("authentication must not restart automatically")
	}
	if len(proxy["gateway-auth"].Ports) != 0 {
		t.Fatal("production auth ingress must not publish a host port")
	}
	for _, port := range proxy["traefik"].Ports {
		if !strings.Contains(port, "TRAEFIK_BIND_ADDR:-127.0.0.1") {
			t.Fatalf("unauthenticated local port is not loopback by default: %s", port)
		}
	}
	if !slices.Contains(proxy["gateway-auth"].Environment, "GATEWAY_UPSTREAM=http://traefik:${PORT}") {
		t.Fatal("tunnel ingress does not proxy to Traefik")
	}
	if !slices.Contains(proxy["gateway-auth"].Networks, "gateway-access") {
		t.Fatal("auth ingress is unreachable from tunnel network")
	}
	if slices.Contains(proxy["docker-socket-proxy"].Networks, "gateway-access") {
		t.Fatal("tunnels can reach the Docker socket proxy directly")
	}
	for _, provider := range []string{"cloudflared", "holesail"} {
		svc := read(provider)[provider]
		if svc.Restart != "no" {
			t.Errorf("%s automatically restarts", provider)
		}
		if !slices.Contains(svc.Networks, "gateway-access") {
			t.Errorf("%s cannot reach auth ingress", provider)
		}
		command := strings.Join(svc.Command, " ")
		if !strings.Contains(command, "gateway-auth") || strings.Contains(command, "traefik") {
			t.Errorf("%s bypasses authentication: %s", provider, command)
		}
	}
	files, err := mount.DevContent.ReadDir("dev/compose")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		data, err := mount.DevContent.ReadFile("dev/compose/" + file.Name() + "/compose.yml")
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "middlewares=${TRAEFIK_AUTH_MIDDLEWARE") {
			t.Errorf("local routes depend on auth in %s", file.Name())
		}
	}
}
