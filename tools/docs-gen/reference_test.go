package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"gitlab.wikimedia.org/repos/releng/cli/commands/tools"
)

func TestKubectlReferenceExclusion(t *testing.T) {
	root := &cobra.Command{Use: "mw"}
	root.AddCommand(tools.NewToolforgeCmd())
	kubernetes, remaining, err := root.Find([]string{"toolforge", "kubernetes", "get", "pods"})
	if err != nil || kubernetes.Name() != "get" || len(remaining) != 1 {
		t.Fatalf("kubectl command unavailable before generation: %v %v", remaining, err)
	}
	pruneReferenceChildren(root)
	dir := t.TempDir()
	if err := doc.GenMarkdownTree(root, dir); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "mw_toolforge_kubernetes_") {
			t.Errorf("generated kubectl subcommand page: %s", file.Name())
		}
	}
	overview, err := os.ReadFile(filepath.Join(dir, "mw_toolforge_kubernetes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overview), "https://kubernetes.io/docs/reference/kubectl/") {
		t.Fatal("overview lost the upstream documentation link")
	}
	for _, name := range []string{"mw_toolforge.md", "mw_toolforge_job.md", "mw_toolforge_build.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing Toolforge reference %s: %v", name, err)
		}
	}
	// The publishing filter must not change newly constructed runtime commands.
	runtime := &cobra.Command{Use: "mw"}
	runtime.AddCommand(tools.NewToolforgeCmd())
	found, _, err := runtime.Find([]string{"toolforge", "kubernetes", "get", "pods"})
	if err != nil || found.Name() != "get" {
		t.Fatalf("runtime kubectl tree changed: %v", err)
	}
}
