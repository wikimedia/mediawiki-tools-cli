package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func TestRenamedHelpAndExamples(t *testing.T) {
	t.Setenv("MWCLI_CONTEXT_TEST", "1")
	previous := cli.Opts
	t.Cleanup(func() { cli.Opts = previous })
	cli.Opts.NoInteraction = true
	for _, name := range []string{"mw", "wm", "wiki-tool"} {
		for _, args := range [][]string{{"--help"}, {"dev", "--help"}, {"cloudvps", "ssh", "--help"}, {"cloudvps", "auth", "login", "--help"}, {"cloudvps", "auth", "add", "--help"}, {"analytics", "pageviews", "--help"}} {
			root := NewRootCmd(name)
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(io.Discard)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if !strings.Contains(text, "Usage:\n  "+name+" ") {
				t.Fatalf("wrong invocation: %s", text)
			}
			if strings.Contains(text, "Additional Commands:") {
				t.Fatal("help still uses the old ungrouped-command heading")
			}
			if len(args) == 1 {
				if !strings.Contains(text, "Other Commands:") {
					t.Fatal("root help is missing Other Commands")
				}
				if !strings.Contains(text, cli.DisplayName) || strings.Contains(text, "+----+") {
					t.Fatal("plain root help lost brand or contains terminal banner")
				}
			} else {
				command, _, err := root.Find(args[:len(args)-1])
				if err != nil {
					t.Fatal(err)
				}
				// Aliases remain callable, while examples use the canonical name.
				if !strings.Contains(text, "  "+command.CommandPath()+" ") {
					t.Fatalf("example is not copyable: %s", text)
				}
			}
		}
	}
	if NewMwCliCmd().Name() != cli.DefaultCommandName {
		t.Fatal("generated docs use runtime executable name")
	}
}

func TestExampleRenderingDoesNotRewriteArgumentValues(t *testing.T) {
	root := &cobra.Command{Use: "wm"}
	leaf := &cobra.Command{Use: "probe", Example: "probe --value 'mwcli'\n# explanation\n"}
	root.AddCommand(leaf)
	if got := cobrautil.CommandExamples(leaf); got != "  wm probe --value 'mwcli'\n  # explanation\n" {
		t.Fatalf("unexpected examples %q", got)
	}
	if leaf.Example != "probe --value 'mwcli'\n# explanation\n" {
		t.Fatal("canonical examples mutated")
	}
}

func TestDevelopmentCommandAliases(t *testing.T) {
	t.Setenv("MWCLI_CONTEXT_TEST", "1")
	for _, name := range []string{"mw", "wm", "wiki-tool"} {
		root := NewRootCmd(name)
		canonical, _, err := root.Find([]string{"dev", "mediawiki", "install"})
		if err != nil {
			t.Fatal(err)
		}
		legacy, _, err := root.Find([]string{"docker", "mediawiki", "install"})
		if err != nil || canonical != legacy {
			t.Fatalf("docker alias did not resolve to dev: %v", err)
		}
		if canonical.CommandPath() != name+" dev mediawiki install" {
			t.Fatalf("unexpected canonical path: %s", canonical.CommandPath())
		}
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"docker", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "development environment") || !strings.Contains(out.String(), name+" dev mediawiki create") {
			t.Fatalf("help does not use the new environment name: %s", out.String())
		}
	}
}

func TestDevDashboardUsesSelectedContextWithoutDocker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", dir)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PORT", "")
	t.Setenv("MWCLI_CONTEXT_TEST", "1")
	previousOptions, previousContext := cli.Opts, devenv.Context
	t.Cleanup(func() { cli.Opts, devenv.Context = previousOptions, previousContext })
	contextDir := filepath.Join(dir, "mwdd", "work")
	if err := os.MkdirAll(contextDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, ".env"), []byte("PORT=8181\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := NewRootCmd("mw")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"dev", "dashboard", "--context", "work", "--no-interaction"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "http://localhost:8181/proxy/dashboard/\n" {
		t.Fatalf("dashboard output = %q", got)
	}
}

func TestDevRecipePreviewAndValidationLeaveContextUntouched(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", dir)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("MWCLI_CONTEXT_TEST", "1")
	previousOptions, previousContext := cli.Opts, devenv.Context
	t.Cleanup(func() { cli.Opts, devenv.Context = previousOptions, previousContext })

	for _, args := range [][]string{
		{"validate", "example-git-clone"},
		{"--dry-run", "example-git-clone"},
		{"--dry-run", "mediasearch"},
	} {
		root := NewRootCmd("mw")
		root.SetArgs(append([]string{"--no-interaction", "dev", "--context", "recipe-preview", "recipe"}, args...))
		if err := root.Execute(); err != nil {
			t.Fatalf("recipe %v: %v", args, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mwdd")); !os.IsNotExist(err) {
		t.Fatalf("recipe commands touched development contexts: %v", err)
	}
}
