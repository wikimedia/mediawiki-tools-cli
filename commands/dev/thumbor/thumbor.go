package thumbor

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed thumbor.long.md
var thumborLong string

func NewCmd() *cobra.Command {
	return devenv.NewServiceCmd("thumbor", devenv.ServiceTexts{Long: thumborLong}, []string{})
}
