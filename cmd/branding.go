package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"golang.org/x/term"
)

func rootBanner(cmd *cobra.Command) bool {
	file, ok := cmd.OutOrStdout().(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return false
	}
	name := cmd.Name()
	border := "+" + strings.Repeat("-", len(name)+2) + "+"
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n| %s |  %s\n%s\n\n", border, name, cli.DisplayName, border)
	return true
}
