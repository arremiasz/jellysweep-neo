package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	radarrAPI "github.com/devopsarr/radarr-go/radarr"
	sonarrAPI "github.com/devopsarr/sonarr-go/sonarr"
	"github.com/jon4hz/jellysweep/internal/api/models"
	"github.com/jon4hz/jellysweep/internal/cache"
	"github.com/jon4hz/jellysweep/internal/config"
	"github.com/jon4hz/jellysweep/internal/database"
	"github.com/jon4hz/jellysweep/internal/engine/arr"
	radarrImpl "github.com/jon4hz/jellysweep/internal/engine/arr/radarr"
	sonarrImpl "github.com/jon4hz/jellysweep/internal/engine/arr/sonarr"
	"github.com/jon4hz/jellysweep/internal/engine/jellyfin"
	"github.com/jon4hz/jellysweep/internal/engine/stats"
	"github.com/jon4hz/jellysweep/internal/engine/stats/jellystat"
	"github.com/jon4hz/jellysweep/internal/notify/email"
	"github.com/jon4hz/jellysweep/internal/notify/ntfy"
	"github.com/jon4hz/jellysweep/internal/notify/webpush"
	"github.com/jon4hz/jellysweep/internal/scheduler"
	"github.com/jon4hz/jellysweep/internal/settings"
	"github.com/jon4hz/jellysweep/internal/tags"
	"github.com/jon4hz/jellysweep/pkg/jellyseerr"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

var (
	// ErrRequestAlreadyProcessed indicates that a keep request has already been processed.
	ErrRequestAlreadyProcessed = errors.New("request already processed")
	// ErrUnkeepableMedia indicates that the specified media item cannot be kept.
	ErrUnkeepableMedia = errors.New("media cannot be kept")
)

// Engine is the main engine for Jellysweep, managing interactions with sonarr, radarr, and other services.
// It runs a cleanup job periodically to remove unwanted media.
type Engine struct {
	cfg        *config.Config
	db         database.DB
	settings   *settings.Store
	jellyfin   *jellyfin.Client
	stats      stats.Statser
	jellyseerr *jellyseerr.Client
	sonarr     arr.Arrer
	radarr     arr.Arrer
	email      *email.NotificationService
	ntfy       *ntfy.Client
	webpush    *webpush.Client
	scheduler  *scheduler.Scheduler

	imageCache *cache.ImageCache
	cache      *cache.EngineCache // Cache for engine-specific data

	// migrate old tag based items to database
	initialDBMigration bool

	data *data
}

// data contains any data collected during the cleanup process.
type data struct {
	// userNotifications tracks which users should be notified about which media items
	userNotifications map[string][]arr.MediaItem // key: user email, value: media items
}

// New creates a new Engine instance.
func New(cfg *config.Config, db database.DB, settingsStore *settings.Store, initialDBMigration bool) (*Engine, error) {
	sched, err := scheduler.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	var statsClient stats.Statser
	if cfg.Jellystat != nil {
		statsClient = jellystat.New(cfg.Jellystat)
	}

	engineCache, err := cache.NewEngineCache(cfg.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine cache: %w", err)
	}

	jellyfinClient := jellyfin.New(cfg, func(libraryName string) bool {
		ls, ok := settingsStore.Library(libraryName)
		return ok && ls.Enabled
	})

	var sonarrClient arr.Arrer
	if cfg.Sonarr != nil {
		sonarrClient = sonarrImpl.NewSonarr(cfg, statsClient, engineCache.SonarrTagsCache)
	} else {
		log.Warn("Sonarr configuration is missing, some features will be disabled")
	}

	var radarrClient arr.Arrer
	if cfg.Radarr != nil {
		radarrClient = radarrImpl.NewRadarr(cfg, statsClient, engineCache.RadarrTagsCache)
	} else {
		log.Warn("Radarr configuration is missing, some features will be disabled")
	}

	var jellyseerrClient *jellyseerr.Client
	if cfg.Jellyseerr != nil {
		jellyseerrClient = jellyseerr.New(cfg.Jellyseerr)
	}

	var emailService *email.NotificationService
	if cfg.Email != nil {
		emailService = email.New(cfg.Email)
	}

	var ntfyClient *ntfy.Client
	if cfg.Ntfy != nil && cfg.Ntfy.Enabled {
		ntfyClient = ntfy.NewClient(cfg.Ntfy)
	}

	var webpushClient *webpush.Client
	if cfg.WebPush != nil && cfg.WebPush.Enabled {
		webpushClient = webpush.NewClient(cfg.WebPush)
	}

	engine := &Engine{
		cfg:                cfg,
		db:                 db,
		settings:           settingsStore,
		initialDBMigration: initialDBMigration,
		jellyfin:           jellyfinClient,
		stats:              statsClient,
		jellyseerr:         jellyseerrClient,
		sonarr:             sonarrClient,
		radarr:             radarrClient,
		email:              emailService,
		ntfy:               ntfyClient,
		webpush:            webpushClient,
		scheduler:          sched,
		data: &data{
			userNotifications: make(map[string][]arr.MediaItem),
		},
		imageCache: cache.NewImageCache("./data/cache/images", db),
		cache:      engineCache,
	}

	if err := engine.setupJobs(); err != nil {
		return nil, fmt.Errorf("failed to setup jobs: %w", err)
	}

	return engine, nil
}

// runCleanupJob is the main cleanup job function.
func (e *Engine) runCleanupJob(ctx context.Context) (err error) {
	log.Info("Starting scheduled cleanup job")

	// Clear all caches to ensure fresh data
	e.cache.ClearAll(ctx)

	if e.initialDBMigration {
		// migrate old tag based items to database
		if err := e.migrateTagsToDatabase(ctx); err != nil {
			log.Error("An error occurred while migrating tags to database")
			return err
		}
	}

	e.removeProtectedExpiredItems(ctx)

	mediaItems, err := e.gatherMediaItems(ctx)
	if err != nil {
		log.Error("failed to gather media items", "error", err)
		return err
	}
	log.Info("Media items gathered successfully")

	if err := e.removeItemsNotFoundAnymore(ctx, mediaItems); err != nil {
		log.Error("An error occurred while removing items not found in Jellyfin")
	}

	if err = e.markForDeletion(ctx, mediaItems); err != nil {
		log.Error("An error occurred while marking media for deletion")
	}

	if err := e.unqueueWatchedShows(ctx); err != nil {
		log.Error("An error occurred while un-queueing watched shows", "error", err)
	}

	// only delete media if there was no previous error
	if err == nil {
		if err := e.cleanupMedia(ctx); err != nil {
			log.Error("An error occurred while deleting media")
			return err
		}
	}

	if err := e.createJellyfinLeavingCollections(ctx); err != nil {
		log.Error("An error occurred while creating Jellyfin leaving collections")
	}

	e.removeItemsFromLeavingCollections(ctx)

	log.Info("Scheduled cleanup job completed")
	return err
}

func (e *Engine) removeProtectedExpiredItems(ctx context.Context) {
	log.Info("Removing media items with expired protection from database")
	mediaItems, err := e.db.GetMediaExpiredProtection(ctx, time.Now())
	if err != nil {
		log.Error("Failed to get media items with expired protection from database", "error", err)
		return
	}
	if len(mediaItems) == 0 {
		log.Debug("No media items with expired protection found in database")
		return
	}
	for _, item := range mediaItems {
		item.DBDeleteReason = database.DBDeleteReasonProtectionExpired

		if err := e.db.DeleteMediaItem(ctx, &item); err != nil {
			log.Error("Failed to remove media item with expired protection from database", "title", item.Title, "jellyfinID", item.JellyfinID, "protectedUntil", item.ProtectedUntil, "error", err)
		}

		// Create history event for protection expiration before deletion
		if err := e.CreateProtectionExpiredEvent(ctx, &item); err != nil {
			log.Error("failed to create protection expired event", "title", item.Title, "error", err)
		}
	}
	log.Info("Media items with expired protection removal process completed")
}

// unqueueWatchedShows removes queued TV-show items from the database when an episode
// has been watched after the item was queued. Watching any episode resets a show's
// lifetime per the user-facing model, so the show should leave the deletion queue.
func (e *Engine) unqueueWatchedShows(ctx context.Context) error {
	mediaItems, err := e.db.GetMediaItems(ctx, false)
	if err != nil {
		return fmt.Errorf("get media items: %w", err)
	}
	for _, item := range mediaItems {
		if item.MediaType != database.MediaTypeTV || item.QueuedAt == nil {
			continue
		}
		watch, err := e.stats.GetWatchInfo(ctx, item.JellyfinID)
		if err != nil {
			log.Error("Failed to get watch info for queued show", "title", item.Title, "jellyfinID", item.JellyfinID, "error", err)
			continue
		}
		if watch.LastPlayed.IsZero() || !watch.LastPlayed.After(*item.QueuedAt) {
			continue
		}
		log.Info("Show watched after queueing, removing from deletion queue", "title", item.Title, "lastPlayed", watch.LastPlayed.Format(time.RFC3339))
		item.DBDeleteReason = database.DBDeleteReasonStreamed
		if err := e.CreateStreamedEvent(ctx, &item); err != nil {
			log.Error("failed to create streamed event", "title", item.Title, "error", err)
		}
		if err := e.db.DeleteMediaItem(ctx, &item); err != nil {
			log.Error("failed to remove un-queued show from database", "title", item.Title, "error", err)
		}
	}
	return nil
}

func (e *Engine) removeItemsNotFoundAnymore(ctx context.Context, mediaItems []arr.MediaItem) error {
	log.Info("Removing items no longer present in Jellyfin from database")

	dbMediaItems, err := e.db.GetMediaItems(ctx, false)
	if err != nil {
		log.Error("Failed to get media items from database", "error", err)
		return err
	}

	jellyfinItemMap := make(map[string]struct{})
	for _, item := range mediaItems {
		jellyfinItemMap[item.JellyfinID] = struct{}{}
	}

	for _, dbItem := range dbMediaItems {
		if _, exists := jellyfinItemMap[dbItem.JellyfinID]; !exists {
			log.Info("Media item no longer present in Jellyfin, removing from database", "title", dbItem.Title, "jellyfinID", dbItem.JellyfinID)
			dbItem.DBDeleteReason = database.DBDeleteReasonMissingInJellyfin

			// Create deletion event for missing items
			if err := e.CreateNotFoundAnymoreEvent(ctx, &dbItem); err != nil {
				log.Error("failed to create not found anymore event", "title", dbItem.Title, "error", err)
			}

			if err := e.db.DeleteMediaItem(ctx, &dbItem); err != nil {
				log.Error("Failed to remove media item no longer present in Jellyfin from database", "title", dbItem.Title, "jellyfinID", dbItem.JellyfinID, "error", err)
				continue
			}
		}
	}

	log.Info("Removed items not found in Jellyfin from database successfully")
	return nil
}

// queueDecision is the result of evaluating an arr.MediaItem against the lifetime model.
// When queue is true the item should enter the deletion queue with the given timing.
type queueDecision struct {
	queue       bool
	reason      string
	importedAt  time.Time
	lastWatched time.Time
	// queueAt is the timestamp recorded as QueuedAt on the DB row.
	// The actual on-disk deletion happens at queueAt + library.DeletionPeriodDays.
	queueAt time.Time
}

func (e *Engine) markForDeletion(ctx context.Context, mediaItems []arr.MediaItem) error {
	// Snapshot the items already tracked in the DB so we don't re-queue them.
	tracked, err := e.db.GetMediaItems(ctx, true)
	if err != nil {
		return fmt.Errorf("read tracked media: %w", err)
	}
	trackedKeys := make(map[string]struct{}, len(tracked))
	for _, t := range tracked {
		trackedKeys[t.JellyfinID] = struct{}{}
	}

	now := time.Now()
	toQueue := make([]arr.MediaItem, 0)
	decisions := make([]queueDecision, 0)

	for _, item := range mediaItems {
		if _, alreadyTracked := trackedKeys[item.JellyfinID]; alreadyTracked {
			continue
		}
		dec, ok := e.shouldQueue(ctx, item, now)
		if !ok {
			continue
		}
		toQueue = append(toQueue, item)
		decisions = append(decisions, dec)
		log.Info("Queueing media item for deletion", "title", item.Title, "library", item.LibraryName, "reason", dec.reason)
	}

	log.Info("Populating requester information")
	toQueue = e.populateRequesterInfo(ctx, toQueue)

	e.data.userNotifications = make(map[string][]arr.MediaItem)
	for _, item := range toQueue {
		if item.RequestedBy != "" {
			e.data.userNotifications[item.RequestedBy] = append(e.data.userNotifications[item.RequestedBy], item)
		}
	}

	if len(toQueue) == 0 {
		log.Info("No media items queued for deletion this run")
		return nil
	}

	if err := e.saveQueuedMediaItems(ctx, toQueue, decisions); err != nil {
		log.Error("failed to save queued media items", "error", err)
		return err
	}
	log.Info("Queued media items saved to database successfully", "count", len(toQueue))

	e.sendEmailNotifications()
	if err := e.sendNtfyDeletionSummary(ctx, toQueue); err != nil {
		log.Error("failed to send ntfy deletion summary", "error", err)
	}
	return nil
}

// shouldQueue evaluates a single arr.MediaItem against its library's lifetime settings
// and returns a queueDecision indicating whether (and why) it should be queued.
func (e *Engine) shouldQueue(ctx context.Context, item arr.MediaItem, now time.Time) (queueDecision, bool) {
	libCfg, ok := e.settings.Library(item.LibraryName)
	if !ok || !libCfg.Enabled {
		return queueDecision{}, false
	}

	importedAt := itemImportedAt(item)
	if importedAt.IsZero() {
		log.Debug("skipping item with no import date", "title", item.Title)
		return queueDecision{}, false
	}

	watch, err := e.stats.GetWatchInfo(ctx, item.JellyfinID)
	if err != nil {
		log.Error("failed to get watch info", "title", item.Title, "jellyfinID", item.JellyfinID, "error", err)
		return queueDecision{}, false
	}

	lifetime := time.Duration(libCfg.LifetimeDays) * 24 * time.Hour

	switch item.MediaType {
	case models.MediaTypeMovie:
		if completedMovie(item, watch, libCfg.CompletionThresholdPct) {
			return queueDecision{
				queue:       true,
				reason:      "watched_completed",
				importedAt:  importedAt,
				lastWatched: watch.LastPlayed,
				queueAt:     now,
			}, true
		}
		if now.Sub(importedAt) >= lifetime {
			return queueDecision{
				queue:       true,
				reason:      "lifetime_expired",
				importedAt:  importedAt,
				lastWatched: watch.LastPlayed,
				queueAt:     now,
			}, true
		}
		return queueDecision{}, false

	case models.MediaTypeTV:
		origin := importedAt
		if !watch.LastPlayed.IsZero() && watch.LastPlayed.After(origin) {
			origin = watch.LastPlayed
		}
		if now.Sub(origin) >= lifetime {
			return queueDecision{
				queue:       true,
				reason:      "lifetime_expired",
				importedAt:  importedAt,
				lastWatched: watch.LastPlayed,
				queueAt:     now,
			}, true
		}
		return queueDecision{}, false
	}

	return queueDecision{}, false
}

// completedMovie reports whether the sum of recorded playback durations for a movie
// meets or exceeds the library's completion-percent threshold of the item's runtime.
// The sum is capped at the runtime so re-watches do not skew the percentage.
func completedMovie(item arr.MediaItem, watch stats.WatchInfo, thresholdPct int) bool {
	runtimeMinutes := item.MovieResource.GetRuntime()
	if runtimeMinutes <= 0 {
		return false
	}
	runtime := time.Duration(runtimeMinutes) * time.Minute
	played := watch.TotalPlayed
	if played <= 0 {
		return false
	}
	if played > runtime {
		played = runtime
	}
	if thresholdPct <= 0 {
		thresholdPct = 90
	}
	pct := int((played * 100) / runtime)
	if pct >= thresholdPct {
		log.Debug("movie completion threshold reached", "title", item.Title, "pct", pct, "threshold", thresholdPct)
		return true
	}
	return false
}

func itemImportedAt(item arr.MediaItem) time.Time {
	switch item.MediaType {
	case models.MediaTypeMovie:
		return item.MovieResource.GetAdded()
	case models.MediaTypeTV:
		return item.SeriesResource.GetAdded()
	}
	return time.Time{}
}

// gatherMediaItems gathers all media items from Jellyfin, Sonarr, and Radarr.
// It merges them into a single collection grouped by library.
func (e *Engine) gatherMediaItems(ctx context.Context) ([]arr.MediaItem, error) {
	jellyfinItems, err := e.jellyfin.GetJellyfinItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get jellyfin items: %w", err)
	}

	var sonarrItems []arr.MediaItem
	if e.sonarr != nil {
		sonarrItems, err = e.sonarr.GetItems(ctx, jellyfinItems)
		if err != nil {
			return nil, fmt.Errorf("failed to get sonarr items: %w", err)
		}
	}

	var radarrItems []arr.MediaItem
	if e.radarr != nil {
		radarrItems, err = e.radarr.GetItems(ctx, jellyfinItems)
		if err != nil {
			return nil, fmt.Errorf("failed to get radarr items: %w", err)
		}
	}

	mediaItems := make([]arr.MediaItem, 0, len(sonarrItems)+len(radarrItems))
	mediaItems = append(mediaItems, sonarrItems...)
	mediaItems = append(mediaItems, radarrItems...)

	return mediaItems, nil
}

func arrMediaToDBMediaItem(item arr.MediaItem) database.Media {
	dbItem := database.Media{
		JellyfinID:  item.JellyfinID,
		LibraryName: item.LibraryName,
		RequestedBy: item.RequestedBy,
	}

	switch item.MediaType {
	case models.MediaTypeTV:
		dbItem.MediaType = database.MediaTypeTV
		dbItem.ArrID = item.SeriesResource.GetId()
		dbItem.Title = item.SeriesResource.GetTitle()
		dbItem.Year = item.SeriesResource.GetYear()
		dbItem.FileSize = item.SeriesResource.Statistics.GetSizeOnDisk()
		dbItem.TvdbId = lo.ToPtr(item.SeriesResource.GetTvdbId())
		dbItem.TmdbId = lo.ToPtr(item.SeriesResource.GetTmdbId())

		for _, img := range item.SeriesResource.GetImages() {
			if img.GetCoverType() == sonarrAPI.MEDIACOVERTYPES_POSTER {
				dbItem.PosterURL = img.GetRemoteUrl()
			}
		}

	case models.MediaTypeMovie:
		dbItem.MediaType = database.MediaTypeMovie
		dbItem.ArrID = item.MovieResource.GetId()
		dbItem.Title = item.MovieResource.GetTitle()
		dbItem.Year = item.MovieResource.GetYear()
		dbItem.FileSize = item.MovieResource.Statistics.GetSizeOnDisk()
		dbItem.TmdbId = lo.ToPtr(item.MovieResource.GetTmdbId())

		for _, img := range item.MovieResource.GetImages() {
			if img.GetCoverType() == radarrAPI.MEDIACOVERTYPES_POSTER {
				dbItem.PosterURL = img.GetRemoteUrl()
			}
		}
	default:
		return database.Media{}
	}

	return dbItem
}

// saveQueuedMediaItems persists the items chosen by shouldQueue into the database,
// stamping QueuedAt, lifetime metadata, and the computed DefaultDeleteAt grace deadline.
func (e *Engine) saveQueuedMediaItems(ctx context.Context, mediaItems []arr.MediaItem, decisions []queueDecision) error {
	dbItems := make([]database.Media, 0, len(mediaItems))
	for i, item := range mediaItems {
		dbItem := arrMediaToDBMediaItem(item)
		if dbItem.Title == "" {
			continue
		}
		dec := decisions[i]
		libCfg, ok := e.settings.Library(item.LibraryName)
		if !ok {
			log.Warn("library settings missing at save time, skipping", "library", item.LibraryName)
			continue
		}
		dbItem.ImportedAt = dec.importedAt
		if !dec.lastWatched.IsZero() {
			lw := dec.lastWatched
			dbItem.LastWatchedAt = &lw
		}
		qa := dec.queueAt
		dbItem.QueuedAt = &qa
		dbItem.QueueReason = dec.reason
		dbItem.DefaultDeleteAt = qa.Add(time.Duration(libCfg.DeletionPeriodDays) * 24 * time.Hour)
		dbItems = append(dbItems, dbItem)
	}

	if len(dbItems) == 0 {
		return nil
	}

	if err := e.db.CreateMediaItems(ctx, dbItems); err != nil {
		return fmt.Errorf("create media items: %w", err)
	}

	for i := range dbItems {
		if err := e.CreatePickedUpEvent(ctx, &dbItems[i]); err != nil {
			log.Error("failed to create picked up event", "title", dbItems[i].Title, "error", err)
		}
	}
	return nil
}

// resetAllTags removes all jellysweep tags from all media in Sonarr and Radarr.
// Legacy: also cleans up any remaining tags.
func (e *Engine) resetAllTags(ctx context.Context, additionalTags []string) error {
	log.Info("Resetting all jellysweep tags...")

	if e.sonarr == nil && e.radarr == nil {
		return fmt.Errorf("no Sonarr or Radarr client configured, cannot reset tags")
	}

	g, ctx := errgroup.WithContext(ctx)
	// Reset Sonarr tags
	if e.sonarr != nil {
		g.Go(func() error {
			log.Info("Removing jellysweep tags from Sonarr series...")
			if err := e.sonarr.ResetTags(ctx, additionalTags); err != nil {
				return fmt.Errorf("failed to reset Sonarr tags: %w", err)
			}
			log.Info("Cleaning up all Sonarr jellysweep tags...")
			if err := e.sonarr.CleanupAllTags(ctx, additionalTags); err != nil {
				return fmt.Errorf("failed to cleanup Sonarr tags: %w", err)
			}
			return nil
		})
	}

	// Reset Radarr tags
	if e.radarr != nil {
		g.Go(func() error {
			log.Info("Removing jellysweep tags from Radarr movies...")
			if err := e.radarr.ResetTags(ctx, additionalTags); err != nil {
				return fmt.Errorf("failed to reset Radarr tags: %w", err)
			}
			log.Info("Cleaning up all Radarr jellysweep tags...")
			if err := e.radarr.CleanupAllTags(ctx, additionalTags); err != nil {
				return fmt.Errorf("failed to cleanup Radarr tags: %w", err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Error(err)
		return fmt.Errorf("error while resetting tags")
	}

	log.Info("All jellysweep tags have been successfully reset!")
	return nil
}

// migrateTagsToDatabase migrates existing jellysweep items to the database based on their tags in Sonarr and Radarr.
func (e *Engine) migrateTagsToDatabase(ctx context.Context) error {
	log.Info("Starting migration of jellysweep tags to database...")

	jellyfinItems, err := e.jellyfin.GetJellyfinItems(ctx)
	if err != nil {
		log.Error("Failed to get jellyfin items for migration", "error", err)
		return err
	}

	legacyitems := make([]arr.MediaItem, 0)
	if e.sonarr != nil {
		sonarrItems, err := e.sonarr.GetItems(ctx, jellyfinItems)
		if err != nil {
			log.Error("Failed to get sonarr items for migration", "error", err)
			return err
		}
		legacyitems = append(legacyitems, sonarrItems...)
	}
	if e.radarr != nil {
		radarrItems, err := e.radarr.GetItems(ctx, jellyfinItems)
		if err != nil {
			log.Error("Failed to get radarr items for migration", "error", err)
			return err
		}
		legacyitems = append(legacyitems, radarrItems...)
	}

	dbItems := make([]database.Media, 0)
	now := time.Now()
	for _, item := range legacyitems {
		mustMigrate := false
		dbItem := arrMediaToDBMediaItem(item)
		if dbItem.Title == "" {
			continue
		}
		dbItem.ImportedAt = itemImportedAt(item)
		for _, tagName := range item.Tags {
			tag, err := tags.ParseJellysweepTag(tagName)
			if err != nil {
				continue
			}

			if !tag.ProtectedUntil.IsZero() {
				dbItem.ProtectedUntil = &tag.ProtectedUntil
				mustMigrate = true
			}
			if tag.MustDelete {
				dbItem.Unkeepable = true
				mustMigrate = true
			}
			if !tag.DeletionDate.IsZero() {
				dbItem.DefaultDeleteAt = tag.DeletionDate
				qa := now
				dbItem.QueuedAt = &qa
				dbItem.QueueReason = "legacy_migration"
				mustMigrate = true
			}
		}

		if mustMigrate {
			dbItems = append(dbItems, dbItem)
			log.Info("Migrating legacy-tagged item to database", "title", dbItem.Title, "library", dbItem.LibraryName)
		}
	}

	if len(dbItems) == 0 {
		log.Debug("No items found for migration")
		return nil
	}

	if err := e.db.CreateMediaItems(ctx, dbItems); err != nil {
		log.Error("Failed to migrate items to database", "error", err)
		return err
	}

	if err := e.resetAllTags(ctx, nil); err != nil {
		log.Error("Failed to reset tags after migration", "error", err)
		return err
	}

	log.Info("Migration of tags to database completed successfully")

	return nil
}
