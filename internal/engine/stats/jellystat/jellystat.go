// Package jellystat is the Statser adapter for the Jellystat provider.
package jellystat

import (
	"context"
	"time"

	"github.com/jon4hz/jellysweep/internal/config"
	"github.com/jon4hz/jellysweep/internal/engine/stats"
	"github.com/jon4hz/jellysweep/pkg/jellystat"
)

type jellystatClient struct {
	client *jellystat.Client
}

// New constructs a Jellystat-backed Statser.
func New(cfg *config.JellystatConfig) stats.Statser {
	return &jellystatClient{
		client: jellystat.New(cfg),
	}
}

func (s *jellystatClient) GetWatchInfo(ctx context.Context, jellyfinID string) (stats.WatchInfo, error) {
	info, err := s.client.GetLastPlayed(ctx, jellyfinID)
	if err != nil {
		return stats.WatchInfo{}, err
	}
	var out stats.WatchInfo
	if info == nil {
		return out, nil
	}
	if info.LastPlayed != nil {
		out.LastPlayed = *info.LastPlayed
	}
	out.SessionCount = info.PlayCount
	out.TotalPlayed = time.Duration(info.TotalRuntime) * time.Second
	return out, nil
}
