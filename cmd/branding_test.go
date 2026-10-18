package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func TestRenamedHelpAndExamples(t *testing.T) {
	t.Setenv("MWCLI_CONTEXT_TEST", "1")
	previous := cli.Opts
	t.Cleanup(func() { cli.Opts = previous })
	cli.Opts.NoInteraction = true
	for _, name := range []string{"mw", "wm", "wiki-tool"} {
		for _, args := range [][]string{{"--help"}, {"cloudvps", "ssh", "--help"}, {"cloudvps", "auth", "add", "--help"}, {"analytics", "pageviews", "--help"}} {
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
			if len(args) == 1 {
				if !strings.Contains(text, cli.DisplayName) || strings.Contains(text, "+----+") {
					t.Fatal("plain root help lost brand or contains terminal banner")
				}
			} else if !strings.Contains(text, "  "+name+" "+strings.Join(args[:len(args)-1], " ")+" ") {
				t.Fatalf("example is not copyable: %s", text)
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
