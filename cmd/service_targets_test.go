package cmd

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func targetTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "mw", ValidArgsFunction: completeServiceTargets, SilenceUsage: true, SilenceErrors: true}
	root.AddGroup(&cobra.Group{ID: "service", Title: "Services"})
	root.PersistentFlags().CountP("verbose", "v", "verbosity")
	root.PersistentFlags().Bool("no-interaction", false, "disable prompts")
	root.PersistentFlags().String("context", "", "context")
	wiki := &cobra.Command{Use: "wiki"}
	wiki.PersistentFlags().String("wiki", "", "API endpoint")
	page := &cobra.Command{Use: "page"}
	page.AddCommand(&cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}})
	wiki.AddCommand(page, &cobra.Command{Use: "file", Run: func(*cobra.Command, []string) {}})
	gerrit := &cobra.Command{Use: "gerrit"}
	gerrit.AddCommand(&cobra.Command{Use: "changes", Run: func(*cobra.Command, []string) {}})
	tools := &cobra.Command{Use: "tools"}
	tools.PersistentFlags().Bool("beta", false, "beta")
	for _, name := range []string{"exec", "cp"} {
		child := &cobra.Command{Use: name, Run: func(*cobra.Command, []string) {}}
		child.Flags().String("tool", "", "tool")
		tools.AddCommand(child)
	}
	root.AddCommand(wiki, gerrit, tools)
	completeTargetHelp(root)
	return root
}

func TestResolveServiceTargetArgs(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"empty", "", ""},
		{"ordinary command", "wiki page list", "wiki page list"},
		{"service alias", "@gerr changes", "gerrit changes"},
		{"site", "@en.wikipedia.org page list", "@en.wikipedia.org page list"},
		{"site alias", "@en.wiki page list", "@en.wikipedia.org page list"},
		{"generic site", "@example.org page list", "wiki --wiki=https://example.org/w/api.php page list"},
		{"unknown", "@unknown", "@unknown"},
		{"invalid host", "@example.org/path page list", "@example.org/path page list"},
		{"global flags", "-vv --no-interaction @en.wiki page list", "-vv --no-interaction @en.wikipedia.org page list"},
		{"flag value", "--context @literal @gerr changes", "--context @literal gerrit changes"},
		{"unknown flag", "--unknown @gerr", "--unknown @gerr"},
		{"flag terminator", "-- @gerr", "-- @gerr"},
		{"partial site", "__complete @en.wiki", "__complete @en.wiki"},
		{"partial service", "__complete @gerr", "__complete @gerr"},
		{"partial with flag", "__complete -v @en.wiki", "__complete -v @en.wiki"},
		{"help", "help @en.wiki page", "help @en.wikipedia.org page"},
		{"help completion", "__complete help @en.wiki", "__complete help @en.wiki"},
		{"tool", "@mytool.toolforge.org exec", "tools exec --tool=mytool"},
		{"tool with flags", "-v @mytool.toolforge.org --beta exec", "-v tools --beta exec --tool=mytool"},
		{"explicit endpoint", "@en.wiki --wiki=https://example.org/api.php page list", "@en.wikipedia.org --wiki=https://example.org/api.php page list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := targetTestRoot()
			args := strings.Fields(tc.input)
			original := append([]string{}, args...)
			got := resolveServiceTargetArgs(root, args)
			if strings.Join(got, " ") != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if !reflect.DeepEqual(args, original) {
				t.Errorf("input changed: %q", args)
			}
			root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
				if flag.Changed {
					t.Errorf("flag %s marked changed", flag.Name)
				}
			})
			if count, _ := root.PersistentFlags().GetCount("verbose"); count != 0 {
				t.Errorf("verbosity changed to %d", count)
			}
		})
	}
}

func TestServiceTargetCompletion(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args, want []string
	}{
		{"site prefix", []string{"__complete", "@en.wiki"}, []string{"@en.wikipedia.org", ":4"}},
		{"service prefix", []string{"__complete", "@gerr"}, []string{"@gerrit.wikimedia.org", ":4"}},
		{"wiki commands", []string{"__complete", "@en.wiki", ""}, []string{"page", "file"}},
		{"wiki with flags", []string{"__complete", "-vv", "@en.wiki", ""}, []string{"page", "file"}},
		{"no descriptions", []string{"__completeNoDesc", "@en.wiki", ""}, []string{"page", "file"}},
		{"gerrit commands", []string{"__complete", "@gerr", ""}, []string{"changes"}},
		{"help target", []string{"__complete", "help", "@en.wiki"}, []string{"@en.wikipedia.org", ":4"}},
		{"help commands", []string{"__complete", "help", "@en.wiki", ""}, []string{"page", "file"}},
		{"unknown target", []string{"__complete", "@unknown"}, []string{":4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := targetTestRoot()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(io.Discard)
			root.SetArgs(resolveServiceTargetArgs(root, tc.args))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				found := false
				for _, line := range strings.Split(output.String(), "\n") {
					if strings.SplitN(line, "\t", 2)[0] == want {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %q in %q", want, output.String())
				}
			}
		})
	}
}

func TestServiceTargetExecution(t *testing.T) {
	root := targetTestRoot()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	args := resolveServiceTargetArgs(root, []string{"-vv", "@en.wiki", "page", "list"})
	page, _, err := root.Find([]string{"@en.wikipedia.org", "page", "list"})
	if err != nil {
		t.Fatal(err)
	}
	page.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if count, _ := root.PersistentFlags().GetCount("verbose"); count != 2 {
		t.Errorf("verbosity = %d, want 2", count)
	}
	if endpoint, _ := page.Flags().GetString("wiki"); endpoint != "https://en.wikipedia.org/w/api.php" {
		t.Errorf("endpoint = %q", endpoint)
	}
}
