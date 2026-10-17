package mediawiki

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestInstallComposerDependencies(t *testing.T) {
	for _, failAt := range []int{0, 1, 2} {
		calls := 0
		err := installComposerDependencies(func(command []string) int {
			calls++
			action := "update"
			if calls == 2 {
				action = "install"
			}
			if !reflect.DeepEqual(command, composerCommand(action, "--ignore-platform-reqs", "--no-interaction")) {
				t.Fatalf("unexpected command %v", command)
			}
			if calls == failAt {
				return 17
			}
			return 0
		})
		if failAt == 0 && (err != nil || calls != 2) {
			t.Fatalf("success: calls=%d err=%v", calls, err)
		}
		if failAt != 0 && (err == nil || calls != failAt || !strings.Contains(err.Error(), "exit code 17")) {
			t.Fatalf("failure %d: calls=%d err=%v", failAt, calls, err)
		}
	}
}

func TestComposerCommandHomeAndArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX container shell test")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "composer"), []byte("#!/bin/sh\nprintf '%s\\n' \"$HOME\" \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, home := range []string{"", "/custom home"} {
		command := composerCommand("config", "argument with spaces", "$(must-not-run)")
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = []string{"PATH=" + dir + ":" + os.Getenv("PATH")}
		if home != "" {
			cmd.Env = append(cmd.Env, "HOME="+home)
		}
		output, err := cmd.CombinedOutput()
		wantHome := home
		if wantHome == "" {
			wantHome = "/"
		}
		want := wantHome + "\nconfig\nargument with spaces\n$(must-not-run)\n"
		if err != nil || string(output) != want {
			t.Fatalf("HOME=%q: output=%q err=%v", home, output, err)
		}
	}
}
