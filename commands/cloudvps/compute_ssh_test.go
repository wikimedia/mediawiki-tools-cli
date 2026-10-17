package cloudvps

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSSHScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	for _, exitCode := range []int{0, 7} {
		t.Run(fmt.Sprint(exitCode), func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "ssh.log")
			t.Setenv("SSH_TEST_LOG", log)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			// Simulate the remote shell locally; no SSH connection is made.
			writeTestExecutable(t, filepath.Join(dir, "ssh"), "#!/bin/bash\nprintf '%s\\n' \"$@\" >> \"$SSH_TEST_LOG\"\nexec sh -c \"${!#}\"\n")
			writeTestExecutable(t, filepath.Join(dir, "sudo"), "#!/bin/sh\n[ \"$1\" = -n ] && [ \"$2\" = -- ] || exit 99\nshift 2\nexec \"$@\"\n")
			path := filepath.Join(dir, "setup with spaces.sh")
			script := fmt.Sprintf("#!/bin/bash\nset -eu\nprintf '%%s\\n' \"$0\" \"$1\" \"$2\"\nread -r input\nprintf '%%s\\n' \"$input\"\nexit %d\n", exitCode)
			if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			provider := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/compute/servers/instance-id" {
					t.Errorf("unexpected API request: %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"server":{"id":"instance-id","name":"ci-runner"}}`)
			})
			cmd := newComputeSSHCmd(provider)
			cmd.SetIn(strings.NewReader("interactive input\n"))
			literal := "it's literal; $(touch " + filepath.Join(dir, "injected") + ")"
			output, err := executeTestCommand(cmd, "instance-id", "--project=mwcli", "--script", path, "--sudo", "--tty", "--", "a b", literal)
			if exitCode == 0 && err != nil {
				t.Fatal(err)
			}
			if exitCode != 0 {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitCode {
					t.Fatalf("expected exit %d, got %v", exitCode, err)
				}
			}
			lines := strings.SplitN(output, "\n", 2)
			if len(lines) != 2 || !remoteScriptPath.MatchString(lines[0]) || lines[1] != "a b\n"+literal+"\ninteractive input\n" {
				t.Fatalf("script arguments or input changed: %q", output)
			}
			if _, err := os.Stat(lines[0]); !os.IsNotExist(err) {
				t.Fatalf("remote script was not cleaned up: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "injected")); !os.IsNotExist(err) {
				t.Fatal("script argument executed as shell code")
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(calls), "\n-t\n") != 1 {
				t.Fatalf("expected a TTY only for execution, got %s", calls)
			}
		})
	}
}

func writeTestExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestSSHScriptValidation(t *testing.T) {
	for _, args := range [][]string{
		{"instance-id", "--sudo"},
		{"instance-id", "--script", filepath.Join(t.TempDir(), "missing.sh")},
	} {
		if _, err := executeTestCommand(newComputeSSHCmd(nil), args...); err == nil {
			t.Fatalf("expected invalid script options to fail: %v", args)
		}
	}
}
