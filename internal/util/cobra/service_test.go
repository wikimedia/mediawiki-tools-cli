package cobrautil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestServiceInfoPreservesCommandAnnotations(t *testing.T) {
	cmd := &cobra.Command{Use: "example", Annotations: map[string]string{"mwcli-docs-skip-children": "upstream"}}
	if CommandServiceInfo(cmd).Name != "example" {
		t.Fatal("service without metadata must retain its command name")
	}
	want := ServiceInfo{Name: "Example", URL: "https://example.org", Logo: "https://example.org/logo.svg"}
	if WithServiceInfo(cmd, want) != cmd || CommandServiceInfo(cmd) != want {
		t.Fatal("service metadata did not round-trip")
	}
	if cmd.Annotations["mwcli-docs-skip-children"] != "upstream" {
		t.Fatal("service metadata replaced an existing command annotation")
	}
}
