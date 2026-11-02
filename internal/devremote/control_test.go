package devremote

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPlatformControlChannel(t *testing.T) {
	address := controlAddress(t.TempDir())
	listener, err := listenControl(address)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("unexpected control request %s", r.URL.Path)
		}
		io.WriteString(w, "ready")
	})}
	go server.Serve(listener)
	defer server.Close()
	client := socketClient(address)
	client.Timeout = 3 * time.Second
	defer client.CloseIdleConnections()
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://session/status", nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "ready" {
		t.Fatalf("control response %q", body)
	}
}

// Run in a child process to exercise the actual platform detachment flags.
func TestDetachedControlHelper(t *testing.T) {
	address := os.Getenv("WM_TEST_CONTROL_ADDRESS")
	if address == "" {
		t.Skip("subprocess helper")
	}
	listener, err := listenControl(address)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{ReadHeaderTimeout: time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ready")
		if r.URL.Path == "/stop" {
			go server.Shutdown(context.Background())
		}
	})
	if err := server.Serve(listener); err != http.ErrServerClosed {
		t.Fatal(err)
	}
}

func TestDetachedSessionControlLifecycle(t *testing.T) {
	directory := t.TempDir()
	address := controlAddress(directory)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(directory, "session.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	child := exec.Command(executable, "-test.run=^TestDetachedControlHelper$")
	child.Env = append(os.Environ(), "WM_TEST_CONTROL_ADDRESS="+address)
	child.Stdout, child.Stderr = log, log
	detach(child)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	client := socketClient(address)
	client.Timeout = 200 * time.Millisecond
	defer client.CloseIdleConnections()
	ready := false
	for i := 0; i < 100; i++ {
		response, err := client.Get("http://session/status")
		if err == nil {
			response.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		data, _ := os.ReadFile(log.Name())
		t.Fatalf("detached controller did not start: %s", data)
	}
	response, err := client.Post("http://session/stop", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("detached controller did not stop")
	}
}
