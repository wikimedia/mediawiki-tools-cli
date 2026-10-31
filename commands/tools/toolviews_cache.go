package tools

import (
	"context"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/diskcache"
)

func toolviewsNamesCache() *diskcache.Cache[[]string] {
	return diskcache.New[[]string](filepath.Join(xdg.CacheHome, "mwcli", "toolviews-tools.json"), 24*time.Hour)
}

func cachedToolviewsNames(ctx context.Context) ([]string, error) {
	cache := toolviewsNamesCache()
	return cache.Get(ctx, fetchToolviewsTools, func() { cache.StartRefresh("--no-interaction", "tools", "toolviews", "_refresh-cache") })
}

func newToolviewsRefreshCmd() *cobra.Command {
	return &cobra.Command{Use: "_refresh-cache", Short: "Refresh the cached Toolviews tool list", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		defer toolviewsNamesCache().FinishRefresh()
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		_, err := toolviewsNamesCache().Refresh(ctx, fetchToolviewsTools)
		return err
	}}
}
