package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func TestSuspendResumeIntegration(t *testing.T) {
	if os.Getenv("MWCLI_DOCKER_TESTS") != "1" {
		t.Skip("requires explicit Docker integration opt-in")
	}
	original := mwdd.Context
	mwdd.Context = fmt.Sprintf("resume-check-%d", time.Now().UnixNano())
	t.Cleanup(func() { mwdd.Context = original })
	m := mwdd.MWDD{BaseDir: t.TempDir()}
	folder := filepath.Join(m.Directory(), "compose", "fixture")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `services:
  keep:
    image: golang:1.23-alpine
    network_mode: none
    stop_grace_period: 1s
    command: [sleep, '600']
  manual:
    image: golang:1.23-alpine
    network_mode: none
    stop_grace_period: 1s
    command: [sleep, '600']
  one-shot:
    image: golang:1.23-alpine
    network_mode: none
    command: ['true']
`
	if err := os.WriteFile(filepath.Join(folder, "compose.yml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	p := m.DockerCompose()
	t.Cleanup(func() {
		if err := p.Command([]string{"down", "--volumes"}).RunAttached(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if err := p.Command([]string{"up", "-d", "--pull", "never"}).RunAttached(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		exited, err := p.ServicesWithStatus("exited")
		if err != nil {
			t.Fatal(err)
		}
		if reflect.DeepEqual(exited, []string{"one-shot"}) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("one-shot did not exit: %v", exited)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := p.Stop([]string{"manual"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := suspendDevelopmentServices(m, p); err != nil {
			t.Fatal(err)
		}
		if err := suspendDevelopmentServices(m, p); err != nil {
			t.Fatal(err)
		}
		if err := resumeDevelopmentServices(m, p); err != nil {
			t.Fatal(err)
		}
		if err := resumeDevelopmentServices(m, p); err != nil {
			t.Fatal(err)
		}
		if err := restartDevelopmentServices(p); err != nil {
			t.Fatal(err)
		}
		running, err := p.ServicesWithStatus("running")
		if err != nil || !reflect.DeepEqual(running, []string{"keep"}) {
			t.Fatalf("cycle %d running=%v err=%v", i, running, err)
		}
		t.Logf("cycle %d: only keep running; manual and one-shot remain stopped", i)
	}
}
