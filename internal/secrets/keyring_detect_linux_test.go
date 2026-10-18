package secrets

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestKeyringDiscoveryDoesNotStartMissingBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing-bus"))
	got := DetectKeyring(context.Background())
	if got.Available {
		t.Fatal("reported a nonexistent keyring as available")
	}
}

func TestKeyringDiscoveryTimesOutDuringAuthentication(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "bus")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-finished // Intentionally never answer the authentication request.
	}()
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+socket)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := DetectKeyring(ctx)
	if got.Available {
		t.Fatal("reported an unresponsive service as available")
	}
	if time.Since(start) > time.Second {
		t.Fatal("keyring discovery blocked on an unresponsive service")
	}
}
