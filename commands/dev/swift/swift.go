package swift

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed swift.long.md
var swiftLong string

func NewCmd() *cobra.Command {
	return devenv.NewServiceCmd("swift", devenv.ServiceTexts{Long: swiftLong}, nil)
}
