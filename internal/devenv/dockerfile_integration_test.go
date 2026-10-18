package devenv

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// Opt in locally: MWCLI_DOCKER_TESTS=1 go test ./internal/devenv -run TestDockerfileRebuildIntegration.
// Uses a cached base image, no published ports and a uniquely named disposable image/project.
func TestDockerfileRebuildIntegration(t *testing.T) {
	if os.Getenv("MWCLI_DOCKER_TESTS") != "1" {
		t.Skip("requires explicit Docker integration opt-in")
	}
	t.Setenv("GITLAB_CI", "1")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	original := Context
	Context = fmt.Sprintf("rebuild-check-%d", time.Now().UnixNano())
	t.Cleanup(func() { Context = original })
	m := DefaultForUser()
	m.EnsureReady()
	m.Env().Set("MEDIAWIKI_VOLUMES_CODE", t.TempDir())
	m.Env().Set("SHELLBOX_SECRET_KEY", "rebuild-test")
	m.Env().Set("NETWORK_SUBNET_PREFIX", "10.254.249")
	m.Env().Set("PORT", "38089")
	service := fmt.Sprintf("custom-aaarebuildcheck%d", time.Now().UnixNano())
	dir := t.TempDir()
	dockerfile := filepath.Join(dir, "Dockerfile")
	base := os.Getenv("MWCLI_DOCKER_TEST_IMAGE")
	if base == "" {
		base = "docker-registry.wikimedia.org/dev/bookworm-php83-fpm:1.0.0"
	}
	if err := osexec.Command("docker", "image", "inspect", base).Run(); err != nil {
		t.Fatalf("base image must already be cached: %v", err)
	}
	runtime := fmt.Sprintf(`services:
  %s:
    image: %s
    network_mode: none
    entrypoint: sh -c
    command: ['while :; do sleep 60; done']
    stop_grace_period: 1s
    volumes: [rebuild-data:/data]
volumes:
  rebuild-data:
`, service, base)
	if err := os.WriteFile(filepath.Join(m.Directory(), service+".yml"), []byte(runtime), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		stdout, stderr, err := m.DockerCompose().Command(args).RunAndCollect()
		if err != nil {
			t.Fatalf("docker compose %v: %v\n%s%s", args, err, stdout.String(), stderr.String())
		}
		return strings.TrimSpace(stdout.String())
	}
	command := func(cmd *cobra.Command, args ...string) {
		t.Helper()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		stdout, stderr, err := m.DockerCompose().Command([]string{"down", "--volumes"}).RunAndCollect()
		if err != nil {
			t.Errorf("cleanup: %v %s%s", err, stdout.String(), stderr.String())
		}
		_ = osexec.Command("docker", "image", "rm", "mwcli-"+service+"-custom:local").Run()
	})
	for i, version := range []string{"first", "second"} {
		content := fmt.Sprintf("FROM %s\nRUN printf '%s' > /tmp/mwcli-override-proof\n", base, version)
		if err := os.WriteFile(dockerfile, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		// Exercise both public commands with their normal defaults.
		if i == 0 {
			if err := writeDockerfileComposeFile(m.Directory(), service, dockerfile); err != nil {
				t.Fatal(err)
			}
			command(NewServiceCreateCmd(service, ""))
			run("exec", "-T", service, "sh", "-c", "printf preserved > /data/proof")
		} else {
			command(NewServiceRebuildCmd(service, ""))
		}
		if got := run("exec", "-T", service, "cat", "/tmp/mwcli-override-proof"); got != version {
			t.Fatalf("got %q want %q", got, version)
		}
		if got := run("exec", "-T", service, "cat", "/data/proof"); got != "preserved" {
			t.Fatalf("volume data lost: %q", got)
		}
	}
	serviceCmd := &cobra.Command{Use: service}
	serviceCmd.AddCommand(NewImageCmd(service))
	command(serviceCmd, "image", "dockerfile", "reset")
	command(NewServiceCreateCmd(service, ""), "--force-recreate")
	run("exec", "-T", service, "test", "!", "-e", "/tmp/mwcli-override-proof")
	if got := run("exec", "-T", service, "cat", "/data/proof"); got != "preserved" {
		t.Fatalf("reset lost volume data: %q", got)
	}
	if _, err := os.Stat(dockerfile); err != nil {
		t.Fatalf("reset removed the Dockerfile: %v", err)
	}
}
