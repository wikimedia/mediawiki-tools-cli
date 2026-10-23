package redis

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

//go:embed redis.long.md
var redisLong string

func NewCmd() *cobra.Command {
	redis := devenv.NewServiceCmd("redis", devenv.ServiceTexts{Long: redisLong}, []string{})
	redis.AddCommand(devenv.NewServiceCommandCmd("redis", []string{"redis-cli"}, []string{"cli"}))
	return redis
}
