package dev

import (
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func TestRecipeStatusHintNamesExistingCommand(t *testing.T) {
	previous := devenv.Context
	t.Cleanup(func() { devenv.Context = previous })

	root := &cobra.Command{Use: "wm"}
	dev := &cobra.Command{Use: "dev"}
	status := newDevStatusCmd()
	dev.AddCommand(status)
	root.AddCommand(dev)

	devenv.Context = "default"
	if got := devContextCommand(dev, "status"); got != "wm dev status" {
		t.Fatalf("status hint = %q", got)
	}
	found, args, err := root.Find([]string{"dev", "status"})
	if err != nil || found != status || len(args) != 0 {
		t.Fatalf("status command = %v, args = %v, err = %v", found, args, err)
	}
	devenv.Context = "work"
	if got := devContextCommand(dev, "status"); got != `wm dev --context "work" status` {
		t.Fatalf("status hint = %q", got)
	}
}
