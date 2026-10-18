package wdqs

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed wdqs.long.md
var wdqsLong string

func NewCmd() *cobra.Command {
	wdqs := devenv.NewServiceCmd("wdqs", devenv.ServiceTexts{Long: wdqsLong}, []string{})
	return wdqs
}
