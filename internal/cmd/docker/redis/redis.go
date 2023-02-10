package redis

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed redis.long.md
var redisLong string

func NewCmd() *cobra.Command {
	redis := mwdd.NewServiceCmd("redis", mwdd.ServiceTexts{Long: redisLong}, []string{})
	redis.AddCommand(mwdd.NewServiceCommandCmd("redis", []string{"redis-cli"}, []string{"cli"}))
	return redis
}
