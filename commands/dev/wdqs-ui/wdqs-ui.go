package wdqsUi

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed wdqs-ui.long.md
var wdqsUiLong string

func NewCmd() *cobra.Command {
	wdqsUi := devenv.NewServiceCmd("wdqs-ui", devenv.ServiceTexts{Long: wdqsUiLong}, []string{})
	return wdqsUi
}
