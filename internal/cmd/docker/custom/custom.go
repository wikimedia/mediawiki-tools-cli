package custom

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed custom.long.md
var customLong string

var customName string = "custom"

func fileFromCustomName() string {
	return mwdd.DefaultForUser().Directory() + "/" + customName + ".yml"
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom",
		Short: "custom docker-compose service sets",
		Long:  cli.RenderMarkdown(customLong),
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	cmd.PersistentFlags().StringVarP(&customName, "name", "n", "custom", "the name of the custom service file, referring to existing docker-compose .yml file in the mwdd directory prefixed with custom-")
	// TODO verify custom names start with "custom-"

	cmd.AddCommand(NewWhereCmd())

	cmd.AddCommand(mwdd.NewImageCmdP(&customName))
	cmd.AddCommand(mwdd.NewServiceCreateCmdP(&customName, ""))
	cmd.AddCommand(mwdd.NewServiceDestroyCmdP(&customName))
	cmd.AddCommand(mwdd.NewServiceStopCmdP(&customName))
	cmd.AddCommand(mwdd.NewServiceStartCmdP(&customName))
	cmd.AddCommand(mwdd.NewServiceExposeCmdP(&customName))
	// There is an expectation that the main service for exec has the same name as the service command overall
	cmd.AddCommand(mwdd.NewServiceExecCmdP(&customName, &customName))

	return cmd
}

func NewWhereCmd() *cobra.Command {
	return mwdd.NewWhereCmd(
		"the custom docker-compose yml file being used",
		func() string { return mwdd.DefaultForUser().Directory() + fileFromCustomName() },
	)
}
