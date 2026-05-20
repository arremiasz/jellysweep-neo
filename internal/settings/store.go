// Package settings owns the runtime, UI-editable settings for Jellysweep.
// Settings live in the database. On first startup, the YAML config is used to seed defaults.
// All reads go through a Store that caches the current values in memory under a RWMutex.
package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"

	"github.com/jon4hz/jellysweep/internal/config"
	"github.com/jon4hz/jellysweep/internal/database"
)

// Store is the in-memory snapshot of UI-editable settings, backed by the database.
type Store struct {
	db database.SettingsDB

	mu        sync.RWMutex
	app       database.AppSettings
	libraries map[string]database.LibrarySettings // key: lowercased library name
}

// New constructs a Store, loading current settings from the database and seeding from YAML on first run.
func New(ctx context.Context, db database.SettingsDB, cfg *config.Config) (*Store, error) {
	s := &Store{
		db:        db,
		libraries: make(map[string]database.LibrarySettings),
	}

	app, err := db.GetAppSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load app settings: %w", err)
	}

	if !app.SeededFromYAML && cfg != nil {
		seedAppFromYAML(app, cfg)
		app.SeededFromYAML = true
		if err := db.SaveAppSettings(ctx, app); err != nil {
			return nil, fmt.Errorf("persist seeded app settings: %w", err)
		}
		log.Info("seeded global settings from YAML config")
	}
	s.app = *app

	rows, err := db.ListLibrarySettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list library settings: %w", err)
	}

	if len(rows) == 0 && cfg != nil && len(cfg.Libraries) > 0 {
		rows = seedLibrariesFromYAML(ctx, db, cfg)
		log.Info("seeded library settings from YAML config", "libraries", len(rows))
	}

	for _, r := range rows {
		s.libraries[strings.ToLower(r.Name)] = r
	}
	return s, nil
}

// App returns a snapshot of the current global app settings.
func (s *Store) App() database.AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.app
}

// Library returns the settings for the given library, or false if unknown.
// Lookup is case-insensitive.
func (s *Store) Library(name string) (database.LibrarySettings, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ls, ok := s.libraries[strings.ToLower(name)]
	return ls, ok
}

// Libraries returns a snapshot of all library settings.
func (s *Store) Libraries() []database.LibrarySettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]database.LibrarySettings, 0, len(s.libraries))
	for _, ls := range s.libraries {
		out = append(out, ls)
	}
	return out
}

// SaveApp persists global settings updates and refreshes the cache.
func (s *Store) SaveApp(ctx context.Context, updated database.AppSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated.ID = 1
	updated.SeededFromYAML = true
	if err := s.db.SaveAppSettings(ctx, &updated); err != nil {
		return err
	}
	s.app = updated
	return nil
}

// UpsertLibrary persists per-library settings and refreshes the cache.
func (s *Store) UpsertLibrary(ctx context.Context, ls database.LibrarySettings) error {
	if strings.TrimSpace(ls.Name) == "" {
		return errors.New("library name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.UpsertLibrarySettings(ctx, &ls); err != nil {
		return err
	}
	s.libraries[strings.ToLower(ls.Name)] = ls
	return nil
}

// DeleteLibrary removes a per-library settings row and the cached entry.
func (s *Store) DeleteLibrary(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.DeleteLibrarySettings(ctx, name); err != nil {
		return err
	}
	delete(s.libraries, strings.ToLower(name))
	return nil
}

func seedAppFromYAML(app *database.AppSettings, cfg *config.Config) {
	app.DryRun = cfg.DryRun
	if cfg.CleanupSchedule != "" {
		app.CleanupSchedule = cfg.CleanupSchedule
	}
	app.LeavingCollectionsEnabled = cfg.LeavingCollectionsEnabled
	app.LeavingCollectionsMovieName = cfg.LeavingCollectionsMovieName
	app.LeavingCollectionsTVName = cfg.LeavingCollectionsTVName
	if cfg.CleanupMode != "" {
		app.CleanupMode = string(cfg.CleanupMode)
	}
	if cfg.KeepCount > 0 {
		app.KeepCount = cfg.KeepCount
	}
}

func seedLibrariesFromYAML(ctx context.Context, db database.SettingsDB, cfg *config.Config) []database.LibrarySettings {
	out := make([]database.LibrarySettings, 0, len(cfg.Libraries))
	for name, lc := range cfg.Libraries {
		if lc == nil {
			continue
		}
		ls := database.LibrarySettings{
			Name:                   name,
			Enabled:                lc.Enabled,
			LifetimeDays:           lc.GetContentAgeThreshold(),
			DeletionPeriodDays:     lc.GetCleanupDelay(),
			ProtectionDays:         lc.GetProtectionPeriod(),
			CompletionThresholdPct: 90,
		}
		if err := db.UpsertLibrarySettings(ctx, &ls); err != nil {
			log.Error("failed to seed library settings", "library", name, "error", err)
			continue
		}
		out = append(out, ls)
	}
	return out
}

// IsNotFound reports whether the given error is a GORM "record not found" error,
// useful for callers reacting to a missing library settings row.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
