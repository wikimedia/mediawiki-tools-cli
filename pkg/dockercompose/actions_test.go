package dockercompose

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestServicesWithStatusIgnoresWarningsOnSuccess(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	script := "#!/bin/sh\nif [ \"$1\" = compose ] && [ \"$#\" -eq 1 ]; then exit 0; fi\nprintf 'mediawiki\\n'\nprintf 'warning: optional variable is unset\\n' >&2\n"
	if err := os.WriteFile(docker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	project := Project{Name: "test", Directory: dir}
	services, err := project.ServicesWithStatus("running")
	if err != nil {
		t.Fatalf("ServicesWithStatus() error = %v", err)
	}
	if want := []string{"mediawiki"}; !reflect.DeepEqual(services, want) {
		t.Fatalf("ServicesWithStatus() = %v, want %v", services, want)
	}
}
