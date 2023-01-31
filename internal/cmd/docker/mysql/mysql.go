package mysql

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewCmd() *cobra.Command {
	mysql := mwdd.NewServiceCmd("mysql", "", []string{})
	mysql.AddCommand(mwdd.NewServiceCommandCmd("mysql", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	return mysql
}
