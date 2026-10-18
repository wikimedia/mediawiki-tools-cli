package cmd

import (
	"testing"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestCommandTreeFlagInheritance(t *testing.T) {
	previousConfigHome, previousOptions := xdg.ConfigHome, cli.Opts
	t.Cleanup(func() { xdg.ConfigHome, cli.Opts = previousConfigHome, previousOptions })
	xdg.ConfigHome = t.TempDir()
	t.Setenv("CI_PROJECT_DIR", t.TempDir())

	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		func() {
			defer func() {
				if conflict := recover(); conflict != nil {
					t.Fatalf("flag conflict in %s: %v", command.CommandPath(), conflict)
				}
			}()
			// Exercise the same flag merging as Cobra's docs generator without
			// executing commands or depending on pandoc and docs publication.
			command.InitDefaultHelpFlag()
			command.InheritedFlags()
			command.LocalFlags()
		}()
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(NewMwCliCmd())
}
