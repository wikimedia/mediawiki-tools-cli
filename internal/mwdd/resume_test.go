package mwdd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResumeStateRoundTrip(t *testing.T) {
	original := Context
	Context = "test"
	t.Cleanup(func() { Context = original })
	m := MWDD{BaseDir: t.TempDir()}
	if _, exists, err := m.ResumeServices(); exists || err != nil {
		t.Fatalf("%v %v", exists, err)
	}
	if err := m.SaveResumeServices([]string{"redis", "mysql", "redis"}); err != nil {
		t.Fatal(err)
	}
	names, exists, err := m.ResumeServices()
	if err != nil || !exists || !reflect.DeepEqual(names, []string{"mysql", "redis"}) {
		t.Fatalf("%v %v %v", names, exists, err)
	}
	if err := m.UpdateResumeServices([]string{"mysql"}, false); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateResumeServices([]string{"postgres"}, true); err != nil {
		t.Fatal(err)
	}
	names, _, _ = m.ResumeServices()
	if !reflect.DeepEqual(names, []string{"postgres", "redis"}) {
		t.Fatal(names)
	}
	m.EnsureReady()
	names, _, _ = m.ResumeServices()
	if len(names) != 2 {
		t.Fatal("sync removed runtime state")
	}
	if err := m.SaveResumeServices(nil); err != nil {
		t.Fatal(err)
	}
	names, exists, err = m.ResumeServices()
	if len(names) != 0 || !exists || err != nil {
		t.Fatalf("empty state %v %v %v", names, exists, err)
	}
	if err := m.ClearResumeServices(); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedResumeState(t *testing.T) {
	original := Context
	Context = "test"
	t.Cleanup(func() { Context = original })
	m := MWDD{BaseDir: t.TempDir()}
	if err := os.MkdirAll(m.Directory(), 0700); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"invalid", `{"version":99}`, `{"version":1,"services":["--help"]}`} {
		if err := os.WriteFile(filepath.Join(m.Directory(), "state-suspended-services.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := m.ResumeServices(); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
	if err := m.SaveResumeServices([]string{"../bad"}); err == nil {
		t.Fatal("accepted invalid service name")
	}
}
