package dev

import (
	"os"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func TestContextEnvironmentAndExplicitFlag(t *testing.T) {
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	original := devenv.Context
	t.Cleanup(func() { devenv.Context = original })
	for _, tc := range []struct{ name, environment, flag, want string }{
		{"default", "", "", "default"},
		{"environment", "work", "", "work"},
		{"explicit", "work", "other", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CONTEXT", tc.environment)
			if tc.environment == "" {
				if err := os.Unsetenv("CONTEXT"); err != nil {
					t.Fatal(err)
				}
			}
			cmd := NewCmd()
			if tc.flag != "" {
				if err := cmd.PersistentFlags().Set("context", tc.flag); err != nil {
					t.Fatal(err)
				}
			}
			if devenv.Context != tc.want {
				t.Fatalf("got %q, want %q", devenv.Context, tc.want)
			}
		})
	}
}
