package tunnel

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func TestProviderSelection(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"", "cloudflare"}, {"cloudflare", "cloudflare"}, {"holesail", "holesail"}, {"invalid", ""},
	} {
		got, err := selectProvider(tc.name)
		if tc.want == "" {
			if err == nil {
				t.Errorf("accepted invalid provider %q", tc.name)
			}
		} else if err != nil || got.name != tc.want {
			t.Errorf("provider(%q)=%q,%v; want %q", tc.name, got.name, err, tc.want)
		}
	}
}

func TestActiveProviderDetection(t *testing.T) {
	for _, tc := range []struct {
		name, flag, want string
		running          []string
		wantErr          bool
	}{
		{name: "nothing running"},
		{name: "local stack only", running: []string{"traefik", "mysql", "gateway-auth"}},
		{name: "Cloudflare", running: []string{"cloudflared", "gateway-auth"}, want: "cloudflare"},
		{name: "Holesail", running: []string{"holesail", "gateway-auth"}, want: "holesail"},
		{name: "explicit stopped logs", flag: "holesail", want: "holesail"},
		{name: "ambiguous", running: []string{"cloudflared", "holesail"}, wantErr: true},
		{name: "explicit selection", flag: "holesail", running: []string{"cloudflared", "holesail"}, want: "holesail"},
		{name: "invalid selection", flag: "invalid", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := activeProvider(tc.flag, tc.running)
			if (err != nil) != tc.wantErr || got.name != tc.want {
				t.Fatalf("got %q, %v; want %q, error=%v", got.name, err, tc.want, tc.wantErr)
			}
		})
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
		p, err := selectProvider(tc.name)
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
	p, _ := selectProvider("cloudflare")
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
	p, _ := selectProvider("cloudflare")
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
	p, _ := selectProvider("holesail")
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

func TestCommandsDetectActiveProviderInsteadOfSavedPreference(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	previous := devenv.Context
	devenv.Context = "active-provider-test"
	t.Cleanup(func() { devenv.Context = previous })
	instance := devenv.DefaultForUser()
	instance.EnsureReady()
	instance.Env().Set("TUNNEL_PROVIDER", "cloudflare")
	key := "hs://s000" + strings.Repeat("a1", 32)
	fakeDocker(t, `
case "$*" in
 *"ps --services"*) printf '%s\n' holesail gateway-auth traefik ;;
 *"ps -q holesail"*) echo fixture-container ;;
 *"inspect"*) echo '2026-10-16T12:00:00Z' ;;
 *"logs"*) echo '`+key+`' ;;
esac
`)
	for _, name := range []string{"status", "url"} {
		cmd := NewCmd()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{name})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), key) {
			t.Fatalf("%s did not use active Holesail: %q", name, output.String())
		}
	}
}

func TestNoTunnelStatusDoesNotAssumeCloudflare(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	fakeDocker(t, "exit 0\n")
	cmd := NewCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "No tunnel is running.\n" {
		t.Fatalf("unexpected stopped status: %q", output.String())
	}
}
