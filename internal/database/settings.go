package database

import (
	"context"
	"errors"
	"time"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

// AppSettings holds the global, UI-editable settings for Jellysweep.
// Exactly one row exists; ID is always 1.
type AppSettings struct {
	gorm.Model
	// DryRun mirrors the legacy cfg.DryRun: if true, deletions are not executed.
	DryRun bool `gorm:"not null;default:true"`
	// CleanupSchedule is the cron expression for the cleanup job.
	CleanupSchedule string `gorm:"not null"`
	// LeavingCollectionsEnabled controls creation of "Leaving Soon" collections.
	LeavingCollectionsEnabled bool `gorm:"not null;default:false"`
	// LeavingCollectionsMovieName is the name of the leaving-movies collection.
	LeavingCollectionsMovieName string
	// LeavingCollectionsTVName is the name of the leaving-tv collection.
	LeavingCollectionsTVName string
	// CleanupMode controls TV series cleanup style ("all", "keep_episodes", "keep_seasons").
	CleanupMode string `gorm:"not null;default:'all'"`
	// KeepCount is the number of episodes/seasons to keep when CleanupMode != "all".
	KeepCount int `gorm:"not null;default:1"`
	// SeededFromYAML marks whether initial values were copied from the YAML config.
	SeededFromYAML bool `gorm:"not null;default:false"`
}

// LibrarySettings holds the per-library, UI-editable cleanup settings.
// One row per library; Name is the library's display name as known to Jellyfin.
type LibrarySettings struct {
	gorm.Model
	// Name is the Jellyfin library name; case-insensitive lookup is the caller's job.
	Name string `gorm:"not null;uniqueIndex"`
	// Enabled controls whether this library is swept at all.
	Enabled bool `gorm:"not null;default:true"`
	// LifetimeDays is the number of days from import-date before a movie/show is auto-queued for deletion.
	LifetimeDays int `gorm:"not null;default:90"`
	// DeletionPeriodDays is the grace period (in days) between queueing and actual deletion.
	DeletionPeriodDays int `gorm:"not null;default:30"`
	// ProtectionDays is the number of days an item stays protected after a keep request is approved.
	ProtectionDays int `gorm:"not null;default:90"`
	// CompletionThresholdPct is the playback-progress percent that counts as "watched" for movies (0-100).
	CompletionThresholdPct int `gorm:"not null;default:90"`
}

// GetAppSettings returns the single global settings row, creating it with zero values if absent.
func (c *Client) GetAppSettings(ctx context.Context) (*AppSettings, error) {
	var s AppSettings
	err := c.db.WithContext(ctx).First(&s, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s = AppSettings{
			Model:           gorm.Model{ID: 1},
			DryRun:          true,
			CleanupSchedule: "0 */12 * * *",
			CleanupMode:     "all",
			KeepCount:       1,
		}
		if cerr := c.db.WithContext(ctx).Create(&s).Error; cerr != nil {
			log.Error("failed to create initial app settings", "error", cerr)
			return nil, cerr
		}
		return &s, nil
	}
	if err != nil {
		log.Error("failed to load app settings", "error", err)
		return nil, err
	}
	return &s, nil
}

// SaveAppSettings persists the global settings row. The row must already exist (use GetAppSettings first).
func (c *Client) SaveAppSettings(ctx context.Context, s *AppSettings) error {
	s.ID = 1
	if err := c.db.WithContext(ctx).Save(s).Error; err != nil {
		log.Error("failed to save app settings", "error", err)
		return err
	}
	return nil
}

// GetLibrarySettings returns the settings row for the given library, or gorm.ErrRecordNotFound if missing.
// Lookup is case-insensitive.
func (c *Client) GetLibrarySettings(ctx context.Context, name string) (*LibrarySettings, error) {
	var ls LibrarySettings
	err := c.db.WithContext(ctx).Where("LOWER(name) = LOWER(?)", name).First(&ls).Error
	if err != nil {
		return nil, err
	}
	return &ls, nil
}

// ListLibrarySettings returns all per-library settings rows.
func (c *Client) ListLibrarySettings(ctx context.Context) ([]LibrarySettings, error) {
	var rows []LibrarySettings
	if err := c.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		log.Error("failed to list library settings", "error", err)
		return nil, err
	}
	return rows, nil
}

// UpsertLibrarySettings creates or updates a library's settings row, matching by case-insensitive name.
func (c *Client) UpsertLibrarySettings(ctx context.Context, ls *LibrarySettings) error {
	existing, err := c.GetLibrarySettings(ctx, ls.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil {
		ls.ID = existing.ID
		ls.CreatedAt = existing.CreatedAt
	}
	ls.UpdatedAt = time.Now()
	if err := c.db.WithContext(ctx).Save(ls).Error; err != nil {
		log.Error("failed to upsert library settings", "library", ls.Name, "error", err)
		return err
	}
	return nil
}

// DeleteLibrarySettings removes a library's settings row by name (case-insensitive).
func (c *Client) DeleteLibrarySettings(ctx context.Context, name string) error {
	res := c.db.WithContext(ctx).Where("LOWER(name) = LOWER(?)", name).Delete(&LibrarySettings{})
	if res.Error != nil {
		log.Error("failed to delete library settings", "library", name, "error", res.Error)
		return res.Error
	}
	return nil
}
