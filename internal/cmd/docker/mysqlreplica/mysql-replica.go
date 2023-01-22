package mysqlreplica

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewCmd() *cobra.Command {
	mysqlReplica := mwdd.NewServiceCmd("mysql-replica", "", []string{})
	mysqlReplica.AddCommand(mwdd.NewServiceCommandCmd("mysql-replica", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	return mysqlReplica
}
