package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	if err := writeManual(root, dir, documentationVersion{}); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(filepath.Join(dir, "toolforge", "kubernetes"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "index.md" {
		t.Fatalf("generated kubectl subcommand pages: %+v", files)
	}
	overview, err := os.ReadFile(filepath.Join(dir, "toolforge", "kubernetes", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overview), "https://kubernetes.io/docs/reference/kubectl/") {
		t.Fatal("overview lost the upstream documentation link")
	}
	for _, name := range []string{"toolforge/index.md", "toolforge/job/index.md", "toolforge/build/index.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing Toolforge reference %s: %v", name, err)
		}
	}
	// The reference filter must not change newly constructed runtime commands.
	runtime := &cobra.Command{Use: "mw"}
	runtime.AddCommand(tools.NewToolforgeCmd())
	found, _, err := runtime.Find([]string{"toolforge", "kubernetes", "get", "pods"})
	if err != nil || found.Name() != "get" {
		t.Fatalf("runtime kubectl tree changed: %v", err)
	}
}
