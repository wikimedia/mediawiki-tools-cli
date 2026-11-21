package mediawiki

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func TestInstallStopsWhenLocalSettingsCannotBeRestored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell for the Docker fixture")
	}
	dir := t.TempDir()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	callsFile := filepath.Join(dir, "compose-calls")
	t.Setenv("COMPOSE_CALLS_FILE", callsFile)

	stub := `#!/bin/sh
printf '%s\n' "$*" >> "$COMPOSE_CALLS_FILE"
case " $* " in
    *" mv /var/www/html/w/LocalSettings.php.devenv.bak."*) exit 37 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}

	checkout := t.TempDir()
	for name, content := range map[string]string{
		".gitreview":        "project=mediawiki/core\n",
		"LocalSettings.php": "<?php require_once '/mwdd/MwddSettings.php';\n",
	} {
		if err := os.WriteFile(filepath.Join(checkout, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := devenv.DefaultForUser().Env().Set("MEDIAWIKI_VOLUMES_CODE", checkout); err != nil {
		t.Fatal(err)
	}

	cmd := NewMediaWikiInstallCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--dbtype", "sqlite"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "restore LocalSettings.php after install") {
		t.Fatalf("expected the restore failure, got %v", err)
	}
	calls, err := os.ReadFile(callsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "/maintenance/update.php") {
		t.Fatal("update.php ran after the restore failure")
	}
}
