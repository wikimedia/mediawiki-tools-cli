package docker

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for name, host := range map[string]string{
		"desktop-linux": "tcp://127.0.0.1:12375",
		"rootless":      "tcp://127.0.0.1:12376",
	} {
		err := contexts.CreateOrUpdate(store.Metadata{Name: name, Endpoints: map[string]interface{}{"docker": contextdocker.EndpointMeta{Host: host}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDockerClientContextSelection(t *testing.T) {
	dir := dockerTestConfig(t)
	for _, tc := range []struct{ name, current, context, host, want string }{
		{"saved context", "desktop-linux", "", "", "tcp://127.0.0.1:12375"},
		{"context overrides saved context", "desktop-linux", "rootless", "", "tcp://127.0.0.1:12376"},
		{"host overrides explicit context", "desktop-linux", "rootless", "tcp://127.0.0.1:12377", "tcp://127.0.0.1:12377"},
		{"host overrides missing context", "desktop-linux", "does-not-exist", "tcp://127.0.0.1:12377", "tcp://127.0.0.1:12377"},
		{"host overrides saved context", "desktop-linux", "", "tcp://127.0.0.1:12377", "tcp://127.0.0.1:12377"},
		{"explicit default context", "desktop-linux", "default", "tcp://127.0.0.1:12377", "tcp://127.0.0.1:12377"},
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host := "tcp://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKER_HOST", host)
	err = CheckDockerDaemon()
	if err == nil || !strings.Contains(err.Error(), host) {
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

// T403412: a saved desktop context can point to a user-owned socket instead of
// the default daemon endpoint. Exercise a real connection through that context.
func TestDockerClientNativeEndpoint(t *testing.T) {
	dir := dockerTestConfig(t)
	listener, host := nativeDockerListener(t)
	server := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/_ping" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("API-Version", "1.45")
			w.Header().Set("OSType", "linux")
			fmt.Fprint(w, "OK")
		}),
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Close()
		if err := <-done; err != http.ErrServerClosed {
			t.Error(err)
		}
	})
	contexts := store.New(filepath.Join(dir, "contexts"), dockerContextStoreConfig())
	if err := contexts.CreateOrUpdate(store.Metadata{
		Name: "desktop-linux", Endpoints: map[string]interface{}{
			"docker": contextdocker.EndpointMeta{Host: host},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, current, context, host string }{
		{"saved context with no environment overrides", "desktop-linux", "", ""},
		{"context overrides saved context", "rootless", "desktop-linux", ""},
		{"host overrides missing context", "rootless", "missing-context", host},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"`+tc.current+`"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DOCKER_CONTEXT", tc.context)
			t.Setenv("DOCKER_HOST", tc.host)
			client, err := NewClientFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if client.DaemonHost() != host {
				t.Fatalf("got endpoint %q, want %q", client.DaemonHost(), host)
			}
			if err := CheckDockerDaemon(); err != nil {
				t.Fatalf("cannot ping context endpoint: %v", err)
			}
		})
	}
	server.Close()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"currentContext":"desktop-linux"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := CheckDockerDaemon()
	if err == nil || !strings.Contains(err.Error(), host) {
		t.Fatalf("closed endpoint error lost socket details: %v", err)
	}
}
