package cmd

import (
	"context"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// managedNeedsBootstrap returns true when the managed DB appears uninitialized
// for runtime operations (providers/default agent/channel instances missing).
func managedNeedsBootstrap(stores *store.Stores) bool {
	if stores == nil {
		return false
	}
	ctx := context.Background()

	needsProviders := true
	if stores.Providers != nil {
		if providers, err := stores.Providers.ListProviders(ctx); err == nil && len(providers) > 0 {
			needsProviders = false
		}
	}

	needsDefaultAgent := true
	if stores.Agents != nil {
		if _, err := stores.Agents.GetByKey(ctx, "default"); err == nil {
			needsDefaultAgent = false
		}
	}

	needsChannels := true
	if stores.ChannelInstances != nil {
		if total, err := stores.ChannelInstances.CountInstances(ctx, store.ChannelInstanceListOpts{Limit: 1}); err == nil && total > 0 {
			needsChannels = false
		}
	}

	needsBootstrap := needsProviders || needsDefaultAgent || needsChannels
	if needsBootstrap {
		slog.Info("managed bootstrap check",
			"needs_providers", needsProviders,
			"needs_default_agent", needsDefaultAgent,
			"needs_channels", needsChannels,
		)
	}
	return needsBootstrap
}
