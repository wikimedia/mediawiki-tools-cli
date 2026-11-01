package mediawiki

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func TestQuibbleRecordingArgs(t *testing.T) {
	for _, tc := range []struct {
		name, server, scriptPath, user, password, logDir string
		args                                             []string
	}{
		{
			name: "defaults", server: "http://mediawiki-web:8080", scriptPath: "/w",
			user: "Admin", password: "mwddpassword", logDir: "/workspace/src/tests/selenium/log",
		},
		{
			name: "proxy", server: "http://traefik:6195", scriptPath: "/proxy/second/w",
			user: "OtherAdmin", password: "custom=password", logDir: "/custom logs",
			args: []string{"MW_SERVER=http://traefik:6195", "MW_SCRIPT_PATH=/proxy/second/w", "MEDIAWIKI_USER=OtherAdmin", "MEDIAWIKI_PASSWORD=custom=password", "LOG_DIR=/custom logs"},
		},
		{
			name: "empty path and last override wins", server: "https://example.test", scriptPath: "",
			user: "Admin", password: "mwddpassword", logDir: "/workspace/src/tests/selenium/log",
			args: []string{"MW_SERVER=http://old.test", "MW_SERVER=https://example.test", "MW_SCRIPT_PATH="},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, env := cobrautil.CommandAndEnvFromArgs(quibbleRecordingArgs(tc.args))
			settings := map[string]string{}
			for _, assignment := range env {
				key, value, _ := strings.Cut(assignment, "=")
				if _, exists := settings[key]; exists {
					t.Fatalf("duplicate setting %s", key)
				}
				settings[key] = value
			}
			for key, want := range map[string]string{
				"MW_SERVER": tc.server, "MW_SCRIPT_PATH": tc.scriptPath,
				"MEDIAWIKI_USER": tc.user, "MEDIAWIKI_PASSWORD": tc.password,
				"LOG_DIR": tc.logDir, "CI": "1", "WDIO_USE_NATIVE_FETCH": "1",
			} {
				if settings[key] != want {
					t.Errorf("%s = %q, want %q", key, settings[key], want)
				}
			}
			wantPrefix := []string{
				"quibble", "--skip-zuul", "--skip-deps", "--skip-install", "--db-is-external",
				"--web-backend", "external", "--web-url", tc.server + tc.scriptPath,
				"--log-dir", tc.logDir, "--command",
			}
			if len(command) != len(wantPrefix)+1 || !reflect.DeepEqual(command[:len(wantPrefix)], wantPrefix) {
				t.Fatalf("unexpected Quibble command: %v", command)
			}
		})
	}
}

func TestQuibbleRecordingSpecShellQuoting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX container shell test")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "npm"), []byte("#!/bin/sh\nprintf '%s\\0' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, spec := range [][]string{nil, {"tests/selenium/specs/recentchanges.js"}, {"spec with 'quotes' and $(false); echo unsafe.js"}} {
		command, _ := cobrautil.CommandAndEnvFromArgs(quibbleRecordingArgs(spec))
		cmd := exec.Command("sh", "-c", command[len(command)-1])
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
		output, err := cmd.CombinedOutput()
		want := strings.Join(seleniumRecordingCommand(spec)[1:], "\x00") + "\x00"
		if err != nil || string(output) != want {
			t.Fatalf("spec %v: output=%q want=%q err=%v", spec, output, want, err)
		}
	}
}

func TestQuibbleRecordArgumentValidation(t *testing.T) {
	cmd, _, err := NewMediaWikiQuibbleCmd().Find([]string{"record"})
	if err != nil || cmd.Name() != "record" {
		t.Fatalf("record command not found: %v", err)
	}
	for _, args := range [][]string{nil, {"one.js"}, {"MW_SERVER=https://example.test", "MW_SCRIPT_PATH=", "one.js"}} {
		if err := cmd.ValidateArgs(args); err != nil {
			t.Errorf("valid args %v: %v", args, err)
		}
	}
	if err := cmd.ValidateArgs([]string{"MW_SCRIPT_PATH=/w", "one.js", "two.js"}); err == nil {
		t.Fatal("multiple specs must be rejected")
	}
	if cmd.Flag("user") == nil {
		t.Fatal("record must inherit the container user flag")
	}
}
