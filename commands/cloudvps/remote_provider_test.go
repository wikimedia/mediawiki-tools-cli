package cloudvps

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNamedRemoteResource(t *testing.T) {
	type resource struct{ id, name string }
	identify := func(r resource) (string, string) { return r.id, r.name }
	for _, tc := range []struct {
		name      string
		available []resource
		want      string
		wantError string
	}{
		{"exact match", []resource{{"legacy-id", "VLAN/legacy"}, {"dualstack-id", defaultRemoteNetwork}}, "dualstack-id", ""},
		{"missing", []resource{{"legacy-id", "VLAN/legacy"}}, "", "unavailable"},
		{"duplicate", []resource{{"first", defaultRemoteNetwork}, {"second", defaultRemoteNetwork}}, "", "multiple"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := namedRemoteResource(tc.available, "network", defaultRemoteNetwork, identify)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || !strings.Contains(err.Error(), "--network") {
					t.Fatalf("expected an actionable error, got %v", err)
				}
			} else if err != nil || id != tc.want {
				t.Fatalf("got %q, %v; want %q", id, err, tc.want)
			}
		})
	}
}

func TestRemoteResourceOverridesSkipCatalogLookups(t *testing.T) {
	cmd := &cobra.Command{}
	want := remoteResources{image: "image-id", flavor: "flavor-id", network: "network-id"}
	got, err := resolveRemoteResources(cmd, nil, want)
	if err != nil || got != want {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}
