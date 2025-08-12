package mysql

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed mysql.long.md
var mwddMysqlLong string

func NewCmd() *cobra.Command {
	mysql := mwdd.NewServiceCmd("mysql", mwdd.ServiceTexts{Long: mwddMysqlLong}, []string{})
	mysql.AddCommand(mwdd.NewServiceCommandCmd("mysql", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	return mysql
}
