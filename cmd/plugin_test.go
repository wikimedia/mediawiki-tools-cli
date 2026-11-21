package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test fixture uses a POSIX shell")
	}
	dir := t.TempDir()
	for _, name := range []string{"draft", "builtin", "alias", "help", "completion"} {
		script := "#!/bin/sh\n[ \"$#\" -eq 2 ] && [ \"$1\" = --model ] && [ \"$2\" = 'a b.gguf' ] || exit 99\nexit 17\n"
		if err := os.WriteFile(filepath.Join(dir, "wm-"+name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	root := func() *cobra.Command {
		cmd := &cobra.Command{Use: "wm"}
		cmd.AddCommand(&cobra.Command{Use: "builtin", Aliases: []string{"alias"}})
		return cmd
	}
	if code, handled := runPlugin([]string{"draft", "--model", "a b.gguf"}, dir, root); !handled || code != 17 {
		t.Fatalf("runPlugin returned (%d, %t), want (17, true)", code, handled)
	}
	for _, name := range []string{"builtin", "alias", "help", "completion", "missing", "../draft", "--draft"} {
		if code, handled := runPlugin([]string{name}, dir, root); handled || code != 0 {
			t.Errorf("runPlugin(%q) returned (%d, %t), want (0, false)", name, code, handled)
		}
	}
}
