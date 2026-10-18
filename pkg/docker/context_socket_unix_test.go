//go:build !windows

package docker

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func nativeDockerListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	// Keep the path short: macOS Unix sockets have a small path-length limit.
	dir, err := os.MkdirTemp("/tmp", "mwcli-docker-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "docker.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	return listener, "unix://" + path
}
