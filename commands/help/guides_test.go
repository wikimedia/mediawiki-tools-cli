package help

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/docs"
)

func guideRoot() *cobra.Command {
	root := &cobra.Command{Use: "wm", PersistentPreRun: func(c *cobra.Command, _ []string) {
		if c.Name() != "__complete" {
			panic("help ran application setup")
		}
	}}
	root.AddCommand(&cobra.Command{Use: "probe", Short: "A normal command"}, NewOutputTopicCmd())
	AddGuides(root)
	return root
}

func TestBundledGuides(t *testing.T) {
	t.Setenv("MWCLI_SKIP_RENDER_MARKDOWN", "1")
	manifest, err := docs.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, guide := range manifest.Guides {
		t.Run(guide.Source, func(t *testing.T) {
			root := guideRoot()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			args := append([]string{"help", "guides"}, guide.Topic()...)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			source, err := docs.Files.ReadFile(guide.Source)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(string(source), "\n") {
				if strings.HasPrefix(line, "# ") && !strings.Contains(out.String(), line) {
					t.Fatalf("missing guide heading %q: %s", line, out.String())
				}
			}
			if strings.Contains(out.String(), "<!-- Imported") {
				t.Fatal("import metadata shown as guide text")
			}
		})
	}
}

func TestGuideNavigation(t *testing.T) {
	t.Setenv("MWCLI_SKIP_RENDER_MARKDOWN", "1")
	for _, tc := range []struct {
		args []string
		want string
		fail bool
	}{
		{[]string{"help"}, "guides", false},
		{[]string{"help", "probe"}, "A normal command", false},
		{[]string{"help", "output"}, "--output", false},
		{[]string{"help", "guides", "dev"}, "mysql-dumps", false},
		{[]string{"help", "guides", "dev", "xdebug", "extra"}, "", true},
		{[]string{"help", "guides", "dev", "missing"}, "", true},
		{[]string{"help", "guides", "dev", "--web"}, "", true},
		{[]string{"__complete", "help", "gu"}, "guides", false},
		{[]string{"__complete", "help", "guides", "dev", "my"}, "mysql-dumps", false},
		{[]string{"__complete", "help", "guides", "dev", "xdebug", "--w"}, "--web", false},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			root := guideRoot()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(tc.args)
			err := root.Execute()
			if (err != nil) != tc.fail {
				t.Fatalf("error = %v, output = %s", err, out.String())
			}
			if len(tc.args) > 2 && tc.args[0] == "__complete" && strings.Count(out.String(), tc.want+"\t") != 1 {
				t.Fatalf("completion should appear once: %s", out.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("missing %q: %s", tc.want, out.String())
			}
		})
	}
}

func TestGuideMarkdown(t *testing.T) {
	manifest, err := docs.Load()
	if err != nil {
		t.Fatal(err)
	}
	guide := docs.Guide{Source: "guides/dev/setup.md", Title: "Cli/guide/Test title"}
	source := "[Contexts](contexts.md#name) and [Index](../../index.md).\n" +
		"[External](https://example.org/example.md)\n" +
		"`[Code](contexts.md)`\n```sh\nmw dev --help\n[Code](contexts.md)\n```\n" +
		"~~~sh\n[Code](contexts.md)\n~~~\n<!-- Imported metadata -->"
	got := guideMarkdown(source, guide, manifest, "wm")
	for _, want := range []string{"Contexts (`wm help guides dev contexts`)", "Index (`wm help guides`)", "[External](https://example.org/example.md)", "`[Code](contexts.md)`", "wm dev --help\n[Code](contexts.md)", "~~~sh\n[Code](contexts.md)\n~~~"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "Imported metadata") {
		t.Fatal("metadata retained")
	}
	if got := guideURL(manifest, guide); got != "https://www.mediawiki.org/wiki/Cli/guide/Test%20title" {
		t.Fatal(got)
	}
}
