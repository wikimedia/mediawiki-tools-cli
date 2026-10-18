package mwdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomDashboardMetadata(t *testing.T) {
	original := Context
	Context = "test"
	t.Cleanup(func() { Context = original })
	m := MWDD{BaseDir: t.TempDir()}
	m.Env().EnsureExists()
	m.Env().Set("CUSTOM_HOST", "from-dotenv.local")
	t.Setenv("CUSTOM_HOST", "from-shell.local")
	config := `services:
  custom-web:
    environment:
      VIRTUAL_HOST: ${CUSTOM_HOST:-fallback.local},alias.local
      SECRET_PASSWORD: do-not-export-this
  custom-worker:
    image: busybox
  custom-list:
    environment:
      - VIRTUAL_HOST=${UNSET_DASHBOARD_HOST:-list.local}
`
	t.Setenv("UNSET_DASHBOARD_HOST", "")
	file := filepath.Join(m.Directory(), "custom-example.yml")
	if err := os.WriteFile(file, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.refreshDashboardServices(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.Directory(), "dashboard-services.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "do-not-export-this") {
		t.Fatal("exported secret")
	}
	var got []dashboardService
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].UIHost != "list.local" || got[1].UIHost != "from-shell.local" || got[2].HasUI {
		t.Fatalf("%+v", got)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := m.refreshDashboardServices(); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "[]" {
		t.Fatalf("stale entries %s", data)
	}
}

func TestDashboardMetadataSurvivesEmbeddedSync(t *testing.T) {
	original := Context
	Context = "test"
	t.Cleanup(func() { Context = original })
	m := MWDD{BaseDir: t.TempDir()}
	m.EnsureReady()
	file := filepath.Join(m.Directory(), "custom-example.yml")
	if err := os.WriteFile(file, []byte("services:\n  custom-worker:\n    image: busybox\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.EnsureReady()
	m.EnsureReady()
	data, err := os.ReadFile(filepath.Join(m.Directory(), "dashboard-services.json"))
	if err != nil || !strings.Contains(string(data), "custom-worker") {
		t.Fatalf("metadata=%s err=%v", data, err)
	}
}
