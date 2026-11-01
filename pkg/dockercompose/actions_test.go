package dockercompose

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func startContainersFixture(t *testing.T) (Project, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mock Docker executable is a POSIX shell script")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "commands.log")
	script := `#!/bin/sh
if [ "$1" = compose ] && [ "$#" -eq 1 ]; then exit 0; fi
printf '%s\n' "$*" >> "$MWCLI_START_LOG"
case "$1 $2" in
  'container inspect')
    printf '%s' "$MWCLI_START_LABELS"
    printf '%s' "$MWCLI_INSPECT_ERROR" >&2
    exit "${MWCLI_INSPECT_EXIT:-0}"
    ;;
  'container start')
    printf '%s' "$MWCLI_START_OUTPUT"
    printf '%s' "$MWCLI_START_ERROR" >&2
    exit "${MWCLI_START_EXIT:-0}"
    ;;
  *)
    printf '%s' "$MWCLI_START_IDS"
    printf '%s' "$MWCLI_LIST_ERROR" >&2
    exit "${MWCLI_LIST_EXIT:-0}"
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("MWCLI_START_LOG", log)
	t.Setenv("MWCLI_START_IDS", "id-wiki\nid-web\nid-mysql\n")
	t.Setenv("MWCLI_START_LABELS", "id-wiki mediawiki\nid-web mediawiki-web\nid-mysql mysql\n")
	t.Setenv("MWCLI_START_OUTPUT", "id-wiki\nid-web\nid-mysql\n")
	return Project{Name: "test", Directory: dir}, log
}

func TestStartContainersReportsServicesAndPreservesExactSelection(t *testing.T) {
	project, log := startContainersFixture(t)
	t.Setenv("MWCLI_START_ERROR", "warning: fixture warning\n")
	var stdout, stderr bytes.Buffer
	services := []string{"mediawiki", "mediawiki-web", "mysql"}
	if err := project.StartContainersWithOutput(services, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "Started: mediawiki, mediawiki-web, mysql\n" || stderr.String() != "Starting: mediawiki, mediawiki-web, mysql\nwarning: fixture warning\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	commands := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(commands) != 3 || !strings.HasSuffix(commands[0], "ps --all --quiet mediawiki mediawiki-web mysql") || !strings.HasPrefix(commands[1], "container inspect --format ") || !strings.HasSuffix(commands[1], "id-wiki id-web id-mysql") || commands[2] != "container start id-wiki id-web id-mysql" {
		t.Fatalf("unexpected command selection (must not start dependencies):\n%s", data)
	}
}

func TestStartContainersReportsPartialFailuresAndReplicas(t *testing.T) {
	project, _ := startContainersFixture(t)
	t.Setenv("MWCLI_START_IDS", "id-wiki1\nid-wiki2\nid-mysql\n")
	t.Setenv("MWCLI_START_LABELS", "id-wiki1 mediawiki\nid-wiki2 mediawiki\nid-mysql mysql\n")
	t.Setenv("MWCLI_START_OUTPUT", "id-wiki1\nid-mysql\n")
	t.Setenv("MWCLI_START_EXIT", "1")
	t.Setenv("MWCLI_START_ERROR", "port is already allocated")
	var stdout, stderr bytes.Buffer
	err := project.StartContainersWithOutput([]string{"mediawiki", "mysql"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "start services mediawiki:") || !strings.Contains(err.Error(), "port is already allocated") {
		t.Fatalf("error=%v", err)
	}
	if stdout.String() != "Started: mysql\n" || stderr.String() != "Starting: mediawiki, mysql\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestStartContainersSkipsEmptySelection(t *testing.T) {
	for _, emptyServices := range []bool{false, true} {
		t.Run(map[bool]string{true: "no services", false: "no containers"}[emptyServices], func(t *testing.T) {
			project, log := startContainersFixture(t)
			t.Setenv("MWCLI_START_IDS", "")
			services := []string{"missing-service"}
			if emptyServices {
				services = nil
			}
			var stdout, stderr bytes.Buffer
			if err := project.StartContainersWithOutput(services, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(log)
			if stdout.Len() != 0 || stderr.Len() != 0 || strings.Contains(string(data), "container start") || (emptyServices && len(data) != 0) {
				t.Fatalf("stdout=%q stderr=%q commands=%s", stdout.String(), stderr.String(), data)
			}
		})
	}
}

func TestStartContainersLookupFailuresNeverStart(t *testing.T) {
	for _, tc := range []struct{ name, variable, value, want string }{
		{"list", "MWCLI_LIST_EXIT", "1", "list containers to start for mediawiki, mysql"},
		{"inspect", "MWCLI_INSPECT_EXIT", "1", "identify containers to start for mediawiki, mysql"},
		{"missing label", "MWCLI_START_LABELS", "", "could not identify the service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, log := startContainersFixture(t)
			t.Setenv(tc.variable, tc.value)
			var stdout, stderr bytes.Buffer
			err := project.StartContainersWithOutput([]string{"mediawiki", "mysql"}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
			data, _ := os.ReadFile(log)
			if strings.Contains(string(data), "container start") || stdout.Len() != 0 {
				t.Fatalf("must not start after failed lookup: %s %s", data, stdout.String())
			}
		})
	}
}
