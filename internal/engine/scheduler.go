package engine

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/go-co-op/gocron/v2"
	"github.com/jon4hz/jellysweep/internal/scheduler"
)

// GetScheduler returns the scheduler instance for API access.
func (e *Engine) GetScheduler() *scheduler.Scheduler {
	return e.scheduler
}

// Run starts the engine and all its background jobs.
func (e *Engine) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Start the scheduler
	e.scheduler.Start()

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// Close stops the engine and cleans up resources.
func (e *Engine) Close() error {
	return e.scheduler.Stop()
}

// setupJobs configures all scheduled jobs.
//
// The cleanup schedule is read once at engine startup from the settings store.
// Changing it in the UI persists, but takes effect after a restart.
func (e *Engine) setupJobs() error {
	schedule := e.settings.App().CleanupSchedule
	cleanupJobDef := gocron.CronJob(schedule, false)
	if err := e.scheduler.AddSingletonJob(
		"cleanup",
		"Media Cleanup",
		"Runs the cleanup loop",
		schedule,
		cleanupJobDef,
		e.runCleanupJob,
		true,
	); err != nil {
		return fmt.Errorf("failed to add cleanup job: %w", err)
	}

	// Add job to clear image cache once a week
	clearImageCacheJobDef := gocron.CronJob("0 0 * * 0", false) // Every Sunday at midnight
	if err := e.scheduler.AddSingletonJob(
		"clear_image_cache",
		"Clear Image Cache",
		"Clears the image cache to free up space",
		"0 0 * * 0", // Every Sunday at midnight
		clearImageCacheJobDef,
		func(ctx context.Context) error {
			return e.imageCache.Clear(ctx)
		},
		false, // Not a singleton, can run multiple times
	); err != nil {
		return fmt.Errorf("failed to add clear image cache job: %w", err)
	}

	log.Info("Scheduled jobs configured successfully")
	return nil
}
