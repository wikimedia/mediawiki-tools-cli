package custom

import (
	_ "embed"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed custom.long.md
var customLong string

var customName string = "custom"

func fileFromCustomName() string {
	return customName + ".yml"
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "custom",
		GroupID: "service",
		Short:   "custom docker compose service sets",
		Long:    cli.RenderMarkdown(customLong),
	}

	cmd.PersistentFlags().StringVarP(&customName, "name", "n", "custom", "Name of the custom compose file (without extension), e.g. custom, custom-two, or custom.local")

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Validate the customName, make sure it is either "custom" or prefixed with
		// "custom-" / "custom.".
		if !(customName == "custom" || strings.HasPrefix(customName, "custom-") || strings.HasPrefix(customName, "custom.")) {
			logrus.Warn("customName should be 'custom' or start with 'custom-' or 'custom.'")
		}
	}

	cmd.AddCommand(NewWhereCmd())

	cmd.AddCommand(devenv.NewImageCmdP(&customName))
	cmd.AddCommand(devenv.NewServiceCreateCmdP(&customName, ""))
	cmd.AddCommand(devenv.NewServiceDestroyCmdP(&customName))
	cmd.AddCommand(devenv.NewServiceStopCmdP(&customName))
	cmd.AddCommand(devenv.NewServiceStartCmdP(&customName))
	cmd.AddCommand(devenv.NewServiceExposeCmdP(&customName))
	// There is an expectation that the main service for exec has the same name as the service command overall
	cmd.AddCommand(devenv.NewServiceExecCmdP(&customName, &customName))

	return cmd
}

func NewWhereCmd() *cobra.Command {
	return devenv.NewWhereCmd(
		"the custom docker-compose yml file being used",
		func() string { return filepath.Join(devenv.DefaultForUser().Directory(), fileFromCustomName()) },
	)
}
