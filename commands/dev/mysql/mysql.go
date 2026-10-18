package mysql

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed mysql.long.md
var devMysqlLong string

func NewCmd() *cobra.Command {
	mysql := devenv.NewServiceCmd("mysql", devenv.ServiceTexts{Long: devMysqlLong}, []string{})
	mysql.AddCommand(devenv.NewServiceCommandCmd("mysql", []string{"mysql", "-uroot", "-ptoor"}, []string{"cli"}))
	return mysql
}
