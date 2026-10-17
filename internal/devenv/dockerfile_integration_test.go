package devenv

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt in locally: MWCLI_DOCKER_TESTS=1 go test ./internal/devenv -run TestDockerfileRebuildIntegration.
// Uses a cached base image, no published ports and a uniquely named disposable image/project.
func TestDockerfileRebuildIntegration(t *testing.T) {
	if os.Getenv("MWCLI_DOCKER_TESTS") != "1" {
		t.Skip("requires explicit Docker integration opt-in")
	}
	dir := t.TempDir()
	service := fmt.Sprintf("override-check-%d", time.Now().UnixNano())
	dockerfile := filepath.Join(dir, "Dockerfile")
	base := os.Getenv("MWCLI_DOCKER_TEST_IMAGE")
	if base == "" {
		base = "docker-registry.wikimedia.org/dev/bookworm-php83-fpm:1.0.0"
	}
	if err := osexec.Command("docker", "image", "inspect", base).Run(); err != nil {
		t.Fatalf("base image must already be cached: %v", err)
	}
	if err := writeDockerfileComposeFile(dir, service, dockerfile); err != nil {
		t.Fatal(err)
	}
	compose := []string{"compose", "--project-name", service, "--file", dockerfileComposeFilePath(dir, service), "--file", filepath.Join(dir, "runtime.yml")}
	runtime := fmt.Sprintf("services:\n  %s:\n    network_mode: none\n    command: [sh, -c, 'while :; do sleep 60; done']\n", service)
	if err := os.WriteFile(filepath.Join(dir, "runtime.yml"), []byte(runtime), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		output, err := osexec.Command("docker", append(append([]string{}, compose...), args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker compose %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	t.Cleanup(func() {
		output, err := osexec.Command("docker", append(append([]string{}, compose...), "down", "--volumes")...).CombinedOutput()
		if err != nil {
			t.Errorf("cleanup: %v %s", err, output)
		}
		_ = osexec.Command("docker", "image", "rm", "mwcli-"+service+"-custom:local").Run()
	})
	for _, version := range []string{"first", "second"} {
		content := fmt.Sprintf("FROM %s\nRUN printf '%s' > /tmp/mwcli-override-proof\n", base, version)
		if err := os.WriteFile(dockerfile, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		run("up", "-d", "--build", "--force-recreate", service)
		if got := run("exec", "-T", service, "cat", "/tmp/mwcli-override-proof"); got != version {
			t.Fatalf("got %q want %q", got, version)
		}
	}
}
