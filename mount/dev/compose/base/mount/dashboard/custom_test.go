package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomServicesWithAndWithoutUI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.json")
	t.Setenv("DASHBOARD_SERVICES_FILE", path)
	data := `[{"name":"custom-worker","has_ui":false},{"name":"custom-web","has_ui":true,"ui_host":"web.local"},{"name":"mysql","has_ui":false}]`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	original := dnsLookupHost
	t.Cleanup(func() { dnsLookupHost = original })
	dnsLookupHost = func(ctx context.Context, host string) ([]string, error) {
		if host == "custom-worker" {
			return nil, os.ErrNotExist
		}
		return []string{"10.0.0.2"}, nil
	}
	counts := map[string]int{}
	found := map[string]Service{}
	for _, service := range checkAllServices() {
		counts[service.Name]++
		found[service.Name] = service
	}
	if counts["mysql"] != 1 || counts["custom-worker"] != 1 || counts["custom-web"] != 1 {
		t.Fatalf("%v", counts)
	}
	if found["custom-worker"].Running || found["custom-worker"].HasUI {
		t.Fatal(found["custom-worker"])
	}
	if !found["custom-web"].Running || found["custom-web"].UIURL == "" {
		t.Fatal(found["custom-web"])
	}
}

func TestMalformedCustomServicesRetainsBundled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.json")
	t.Setenv("DASHBOARD_SERVICES_FILE", path)
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if len(serviceConfigs()) != len(servicesToCheck) {
		t.Fatal("lost bundled services")
	}
}
