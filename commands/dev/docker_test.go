package dev

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func TestFileOnlyCommandsWithoutDocker(t *testing.T) {
	previousOptions := cli.Opts
	previousContext := devenv.Context
	t.Cleanup(func() {
		cli.Opts = previousOptions
		devenv.Context = previousContext
	})

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"env-set", []string{"env", "set", "EXAMPLE", "value"}},
		{"get-code", []string{"mediawiki", "get-code", "--core", "--dry-run", "--gerrit-interaction-type", "http"}},
		{"get-code-alias", []string{"mw", "get-code", "--core", "--dry-run", "--gerrit-interaction-type", "http"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITLAB_CI", "true")
			t.Setenv("CI_PROJECT_DIR", t.TempDir())
			t.Setenv("MWCLI_MEDIAWIKI_ENV_COMMAND", "")
			t.Setenv("MW_DOCKER_MEDIAWIKI_GET_CODE", "")
			// Git is required even for dry-run; no Docker or Compose is available.
			bin := t.TempDir()
			gitName := "git"
			if runtime.GOOS == "windows" {
				gitName += ".exe"
			}
			if err := os.WriteFile(filepath.Join(bin, gitName), nil, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			cli.Opts.NoInteraction = true
			cmd := NewCmd()
			env := devenv.DefaultForUser().Env()
			env.Set("PORT", "8080")
			env.Set("MEDIAWIKI_VOLUMES_CODE", t.TempDir())
			env.Set("MEDIAWIKI_VOLUMES_DOT_COMPOSER", t.TempDir())
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if tc.name == "env-set" && env.Get("EXAMPLE") != "value" {
				t.Fatal("environment variable was not written")
			}
		})
	}
}
