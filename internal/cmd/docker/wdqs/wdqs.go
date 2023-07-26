package wdqs

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed wdqs.long.md
var wdqsLong string

func NewCmd() *cobra.Command {
	wdqs := mwdd.NewServiceCmd("wdqs", mwdd.ServiceTexts{Long: wdqsLong}, []string{})
	return wdqs
}
