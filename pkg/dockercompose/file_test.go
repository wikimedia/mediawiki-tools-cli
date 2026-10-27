package dockercompose

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestContentsAcceptsBothDependencyFormats(t *testing.T) {
	for _, tc := range []struct{ name, dependencies string }{
		{"list", "    depends_on:\n      - authentication\n"},
		{"conditions", "    depends_on:\n      authentication:\n        condition: service_healthy\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "compose.yml")
			contents := "services:\n  gateway:\n    image: traefik:test\n" + tc.dependencies + "volumes:\n  data:\n"
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			parsed := File(path).Contents()
			if !reflect.DeepEqual(parsed.ServiceNames(), []string{"gateway"}) || parsed.Services["gateway"].Image != "traefik:test" || !reflect.DeepEqual(parsed.VolumeNames(), []string{"data"}) {
				t.Fatalf("incorrect Compose metadata: %+v", parsed)
			}
		})
	}
}

func TestStartServiceNamesRespectsProfiles(t *testing.T) {
	contents := Contents{Services: map[string]ServiceContents{
		"traefik": {}, "docker-socket-proxy": {}, "gateway-auth": {Profiles: []string{"gateway", "tunnel"}},
	}}
	got := contents.StartServiceNames("traefik")
	sort.Strings(got)
	if want := []string{"docker-socket-proxy", "traefik"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	contents = Contents{Services: map[string]ServiceContents{"holesail": {Profiles: []string{"gateway"}}}}
	if got := contents.StartServiceNames("holesail"); !reflect.DeepEqual(got, []string{"holesail"}) {
		t.Fatalf("explicit profiled service missing: %v", got)
	}
	if got := contents.StartServiceNames("unrelated"); len(got) != 0 {
		t.Fatalf("unrequested optional services selected: %v", got)
	}
}
