package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	dockerconfig "github.com/docker/cli/cli/config"
	contextdocker "github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
)

func dockerTestConfig(t *testing.T) string {
	t.Helper()
	original := dockerconfig.Dir()
	dir := t.TempDir()
	dockerconfig.SetDir(dir)
	t.Cleanup(func() { dockerconfig.SetDir(original) })
	for _, key := range []string{"DOCKER_CONTEXT", "DOCKER_HOST", "DOCKER_API_VERSION", "DOCKER_TLS_VERIFY", "DOCKER_TLS", "DOCKER_CERT_PATH"} {
		t.Setenv(key, "")
	}
	contexts := store.New(filepath.Join(dir, "contexts"), dockerContextStoreConfig())
	for _, name := range []string{"desktop-linux", "rootless"} {
		err := contexts.CreateOrUpdate(store.Metadata{Name: name, Endpoints: map[string]interface{}{"docker": contextdocker.EndpointMeta{Host: "unix:///tmp/" + name + ".sock"}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDockerClientContextSelection(t *testing.T) {
	dir := dockerTestConfig(t)
	for _, tc := range []struct{ name, current, context, host, want string }{
		{"saved context", "desktop-linux", "", "", "unix:///tmp/desktop-linux.sock"},
		{"explicit context wins", "desktop-linux", "rootless", "unix:///tmp/other.sock", "unix:///tmp/rootless.sock"},
		{"host overrides saved context", "desktop-linux", "", "unix:///tmp/other.sock", "unix:///tmp/other.sock"},
		{"explicit default context", "desktop-linux", "default", "unix:///tmp/other.sock", "unix:///tmp/other.sock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"`+tc.current+`"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DOCKER_CONTEXT", tc.context)
			t.Setenv("DOCKER_HOST", tc.host)
			cli, err := NewClientFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			if cli.DaemonHost() != tc.want {
				t.Fatalf("got %q want %q", cli.DaemonHost(), tc.want)
			}
		})
	}
}

func TestDockerClientInvalidContext(t *testing.T) {
	dockerTestConfig(t)
	t.Setenv("DOCKER_CONTEXT", "does-not-exist")
	if _, err := NewClientFromEnv(); err == nil || !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("missing context error: %v", err)
	}
}

func TestDockerDaemonErrorIdentifiesEndpoint(t *testing.T) {
	dockerTestConfig(t)
	t.Setenv("DOCKER_HOST", "unix://"+filepath.Join(t.TempDir(), "missing.sock"))
	err := CheckDockerDaemon()
	if err == nil || !strings.Contains(err.Error(), "missing.sock") {
		t.Fatalf("lost daemon detail: %v", err)
	}
}

func TestDockerContextTLSValidation(t *testing.T) {
	dir := dockerTestConfig(t)
	contexts := store.New(filepath.Join(dir, "contexts"), dockerContextStoreConfig())
	if err := contexts.ResetEndpointTLSMaterial("desktop-linux", "docker", &store.EndpointTLSData{Files: map[string][]byte{"ca.pem": []byte("invalid certificate")}}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_CONTEXT", "desktop-linux")
	if _, err := NewClientFromEnv(); err == nil || !strings.Contains(err.Error(), "ca.pem") {
		t.Fatalf("TLS validation lost: %v", err)
	}
}

func TestDockerContextSSHHelper(t *testing.T) {
	dir := dockerTestConfig(t)
	contexts := store.New(filepath.Join(dir, "contexts"), dockerContextStoreConfig())
	err := contexts.CreateOrUpdate(store.Metadata{Name: "ssh-context", Endpoints: map[string]interface{}{"docker": contextdocker.EndpointMeta{Host: "ssh://developer@example.invalid"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_CONTEXT", "ssh-context")
	cli, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	// Creating the SSH transport should not dial the remote or use the local socket.
	if strings.HasPrefix(cli.DaemonHost(), "unix:") {
		t.Fatal(cli.DaemonHost())
	}
}
