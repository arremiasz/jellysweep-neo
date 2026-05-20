package stats

import (
	"context"
	"time"
)

// WatchInfo holds aggregated playback information for a single Jellyfin item.
type WatchInfo struct {
	// LastPlayed is the timestamp of the most recent playback session, or zero if never played.
	LastPlayed time.Time
	// MaxSessionDuration is the longest single playback session recorded.
	// For movies it is compared against the item's runtime to compute a completion percentage.
	MaxSessionDuration time.Duration
	// SessionCount is the number of recorded playback sessions.
	SessionCount int
}

// Statser is the interface implemented by stats providers (currently only Jellystat).
type Statser interface {
	// GetWatchInfo returns aggregated playback info for a single Jellyfin item ID.
	// Implementations return a zero-value WatchInfo and a nil error when no history exists.
	GetWatchInfo(ctx context.Context, itemID string) (WatchInfo, error)
}
