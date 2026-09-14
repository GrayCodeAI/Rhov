package cmd

import (
	"context"
	"os"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
)

var (
	refreshCatalogFlag     bool
	skipCatalogRefreshFlag bool
)

func ensureCatalogBeforeAgent(ctx context.Context, strict bool) error {
	opts := rhoconfig.CatalogStartupOptions{
		ForceRefresh:    refreshCatalogFlag,
		SkipAutoRefresh: skipCatalogRefreshFlag,
		VerboseOutput:   refreshCatalogFlag,
	}
	if strict {
		return rhoconfig.PrepareCatalogForSession(ctx, os.Stderr, opts)
	}
	rhoconfig.StartupCatalogPrefetch(ctx)
	return nil
}

func startBackgroundCatalogRefresh(ctx context.Context) {
	if skipCatalogRefreshFlag {
		return
	}
	rhoconfig.ScheduleBackgroundCatalogRefresh(ctx)
}
