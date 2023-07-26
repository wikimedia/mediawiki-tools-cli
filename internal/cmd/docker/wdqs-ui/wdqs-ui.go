package wdqsUi

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed wdqs-ui.long.md
var wdqsUiLong string

func NewCmd() *cobra.Command {
	wdqsUi := mwdd.NewServiceCmd("wdqs-ui", mwdd.ServiceTexts{Long: wdqsUiLong}, []string{})
	return wdqsUi
}
