package streamfilter

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jon4hz/jellysweep/internal/config"
	"github.com/jon4hz/jellysweep/internal/engine/arr"
	"github.com/jon4hz/jellysweep/internal/engine/stats"
	"github.com/jon4hz/jellysweep/internal/filter"
)

// Filter implements the filter.Filterer interface.
type Filter struct {
	cfg   *config.Config
	stats stats.Statser
}

var _ filter.Filterer = (*Filter)(nil)

// New creates a new stream Filter instance.
func New(cfg *config.Config, stats stats.Statser) *Filter {
	return &Filter{
		cfg:   cfg,
		stats: stats,
	}
}

// String returns the name of the filter.
func (f *Filter) String() string { return "Stream Filter" }

// Apply filters media items based on stream-specific keep criteria.
func (f *Filter) Apply(ctx context.Context, mediaItems []arr.MediaItem) ([]arr.MediaItem, error) {
	filteredItems := make([]arr.MediaItem, 0)
	for _, item := range mediaItems {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		watch, err := f.stats.GetWatchInfo(ctx, item.JellyfinID)
		if err != nil {
			log.Error("Failed to get watch info for item", "jellyfinID", item.JellyfinID, "error", err)
			return nil, err
		}
		lastStreamed := watch.LastPlayed
		if lastStreamed.IsZero() {
			filteredItems = append(filteredItems, item) // No last streamed time, mark for deletion
			continue
		}
		// Check if the last streamed time is older than the configured threshold
		libraryConfig := f.cfg.GetLibraryConfig(item.LibraryName)
		if libraryConfig != nil && time.Since(lastStreamed) > time.Duration(libraryConfig.GetLastStreamThreshold())*24*time.Hour {
			log.Debug("including item - last streamed outside threshold", "title", item.Title, "lastStreamed", lastStreamed.Format(time.RFC3339))
			filteredItems = append(filteredItems, item)
			continue
		}
		log.Debug("excluding item due to recent stream", "title", item.Title, "lastStreamed", lastStreamed.Format(time.RFC3339))
	}

	return filteredItems, nil
}
