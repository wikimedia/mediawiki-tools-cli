package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func TestProviderSelection(t *testing.T) {
	for _, tc := range []struct{ flag, configured, want string }{
		{"", "", "cloudflare"},
		{"", "holesail", "holesail"},
		{"cloudflare", "holesail", "cloudflare"},
		{"holesail", "invalid", "holesail"},
		{"invalid", "cloudflare", ""},
		{"", "invalid", ""},
	} {
		got, err := selectProvider(tc.flag, tc.configured)
		if tc.want == "" {
			if err == nil {
				t.Errorf("accepted invalid provider %q/%q", tc.flag, tc.configured)
			}
		} else if err != nil || got.name != tc.want {
			t.Errorf("provider(%q, %q) = %q, %v; want %q", tc.flag, tc.configured, got.name, err, tc.want)
		}
	}
}

func TestProviderConnectionParsing(t *testing.T) {
	key := "hs://s000" + strings.Repeat("a1", 32)
	for _, tc := range []struct{ name, logs, want string }{
		{"cloudflare", "old https://old.trycloudflare.com\nlatest https://new.trycloudflare.com", "https://new.trycloudflare.com"},
		{"holesail", "\x1b[2mConnect with key: " + key + "\x1b[0m", key},
		{"holesail", "hs://0000" + strings.Repeat("a1", 32), ""},
		{"holesail", "hs://s000short", ""},
		{"cloudflare", key, ""},
		{"holesail", "server booting", ""},
	} {
		p, err := selectProvider(tc.name, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := endpointFromLogs(p, tc.logs); got != tc.want {
			t.Errorf("%s parsed %q; want %q", tc.name, got, tc.want)
		}
	}
}

func fakeDocker(t *testing.T, body string) dockercompose.Project {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dockercompose.Project{Name: "tunnel-test", Directory: dir}
}

func TestReadEndpointUsesCurrentContainerStart(t *testing.T) {
	project := fakeDocker(t, `
case "$*" in
  *"compose version"*) exit 0 ;;
  *"ps -q"*) echo fixture-container ;;
  *"inspect"*) echo '2026-10-16T12:00:00Z' ;;
  *"logs"*)
    case "$*" in
      *"--since 2026-10-16T12:00:00Z"*) echo 'https://current.trycloudflare.com' ;;
      *) echo 'https://expired.trycloudflare.com' ;;
    esac ;;
esac
`)
	p, _ := selectProvider("cloudflare", "")
	got, _, err := readEndpoint(context.Background(), project, p)
	if err != nil || got != "https://current.trycloudflare.com" {
		t.Fatalf("endpoint = %q, %v", got, err)
	}
}

func TestReadEndpointRejectsStoppedTunnel(t *testing.T) {
	project := fakeDocker(t, `
case "$*" in
  *"logs"*) echo 'https://expired.trycloudflare.com' ;;
esac
`)
	p, _ := selectProvider("cloudflare", "")
	got, _, err := readEndpoint(context.Background(), project, p)
	if err == nil || got != "" {
		t.Fatalf("returned stopped tunnel endpoint %q, %v", got, err)
	}
}

func TestSwitchStopsOnlyOtherTunnel(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker-args")
	t.Setenv("MWCLI_TEST_DOCKER_ARGS", log)
	project := fakeDocker(t, `
printf '%s\n' "$*" >> "$MWCLI_TEST_DOCKER_ARGS"
case "$*" in
  *"ps --services"*) printf '%s\n' cloudflared holesail traefik mediawiki mysql ;;
esac
`)
	p, _ := selectProvider("holesail", "")
	if err := stopOtherProviders(project, p); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "stop cloudflared") || strings.Contains(string(output), "stop holesail") || strings.Contains(string(output), "stop traefik") {
		t.Fatalf("unexpected switch operations: %s", output)
	}
}
