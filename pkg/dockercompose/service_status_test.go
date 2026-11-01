package dockercompose

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func statusProject(t *testing.T, inspect, config string) Project {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MWCLI_TEST_INSPECT", inspect)
	t.Setenv("MWCLI_TEST_CONFIG", config)
	script := `#!/bin/sh
case "$*" in
 "compose") exit 0 ;;
 *"ps --all --quiet"*) printf '%s\n' fixture ;;
 "container inspect -- fixture") printf '%s\n' "$MWCLI_TEST_INSPECT" ;;
 *" config") printf '%s\n' "$MWCLI_TEST_CONFIG" ;;
 *) printf 'unexpected command: %s\n' "$*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return Project{Name: "status-test", Directory: dir}
}

func TestServiceStatusesCombinesReplicasAndCurrentRoles(t *testing.T) {
	inspect := `[
 {"Config":{"Labels":{"com.docker.compose.service":"mysql"}},"State":{"Status":"exited"}},
 {"Config":{"Labels":{"com.docker.compose.service":"mysql"}},"State":{"Status":"running"}},
 {"Config":{"Labels":{"com.docker.compose.service":"setup"}},"State":{"Status":"exited"}},
 {"Config":{"Labels":{"com.docker.compose.service":"orphan"}},"State":{"Status":"running"}}
]`
	project := statusProject(t, inspect, "services:\n  mysql:\n    image: fixture\n  setup:\n    labels:\n      org.wikimedia.cli.lifecycle: oneshot\n")
	statuses, err := project.ServiceStatuses()
	want := []ServiceStatus{{Name: "mysql", State: "running"}, {Name: "setup", State: "exited", OneShot: true}}
	if err != nil || !reflect.DeepEqual(statuses, want) {
		t.Fatalf("statuses=%+v err=%v", statuses, err)
	}
}

func TestServiceStatusesRejectsMalformedInventory(t *testing.T) {
	for _, data := range []string{"not json", `[{"Config":{"Labels":{}},"State":{"Status":"running"}}]`} {
		project := statusProject(t, data, "services: {}")
		if _, err := project.ServiceStatuses(); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestServiceStatusesEmptyScopeDoesNotInspectAllContainers(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
 "compose"|*"ps --all --quiet"*) exit 0 ;;
 *) echo 'inspected an empty scope' >&2;exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	statuses, err := (Project{Name: "empty", Directory: dir}).ServiceStatuses()
	if err != nil || len(statuses) != 0 {
		t.Fatalf("statuses=%v err=%v", statuses, err)
	}
}

func TestServiceStatusesDoesNotHideConfigFailure(t *testing.T) {
	project := statusProject(t, `[{"Config":{"Labels":{"com.docker.compose.service":"mysql"}},"State":{"Status":"running"}}]`, "services: [invalid")
	if _, err := project.ServiceStatuses(); err == nil || !strings.Contains(err.Error(), "parse compose config") {
		t.Fatal(err)
	}
}
