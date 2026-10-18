package mysqlreplica

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func NewCmd() *cobra.Command {
	mysqlReplica := devenv.NewServiceCmd("mysql-replica", devenv.ServiceTexts{}, []string{})
	mysqlReplica.AddCommand(devenv.NewServiceCommandCmd("mysql-replica", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	return mysqlReplica
}
