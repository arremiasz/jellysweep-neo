package handler

import (
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/jon4hz/jellysweep/internal/api/models"
	"github.com/jon4hz/jellysweep/internal/database"
	"github.com/jon4hz/jellysweep/web/templates/pages"
	"github.com/robfig/cron/v3"
)

type appSettingsRequest struct {
	DryRun                      bool   `json:"dryRun"`
	CleanupSchedule             string `json:"cleanupSchedule" binding:"required"`
	LeavingCollectionsEnabled   bool   `json:"leavingCollectionsEnabled"`
	LeavingCollectionsMovieName string `json:"leavingCollectionsMovieName"`
	LeavingCollectionsTVName    string `json:"leavingCollectionsTvName"`
	CleanupMode                 string `json:"cleanupMode" binding:"required,oneof=all keep_episodes keep_seasons"`
	KeepCount                   int    `json:"keepCount" binding:"min=1"`
}

type librarySettingsRequest struct {
	Enabled                bool `json:"enabled"`
	LifetimeDays           int  `json:"lifetimeDays" binding:"min=1"`
	DeletionPeriodDays     int  `json:"deletionPeriodDays" binding:"min=0"`
	ProtectionDays         int  `json:"protectionDays" binding:"min=0"`
	CompletionThresholdPct int  `json:"completionThresholdPct" binding:"min=1,max=100"`
}

type libraryListItem struct {
	Name                   string `json:"name"`
	Enabled                bool   `json:"enabled"`
	LifetimeDays           int    `json:"lifetimeDays"`
	DeletionPeriodDays     int    `json:"deletionPeriodDays"`
	ProtectionDays         int    `json:"protectionDays"`
	CompletionThresholdPct int    `json:"completionThresholdPct"`
}

type settingsResponse struct {
	App       appSettingsRequest `json:"app"`
	Libraries []libraryListItem  `json:"libraries"`
}

// SettingsPanel renders the admin settings page.
func (h *AdminHandler) SettingsPanel(c *gin.Context) {
	user := getUser(c)
	if user == nil {
		return
	}
	app := h.settings.App()
	libs := h.settings.Libraries()
	c.Header("Content-Type", "text/html")
	if err := pages.SettingsPanel(user, toPageApp(app), toPageLibraries(libs), app.DryRun).Render(c.Request.Context(), c.Writer); err != nil {
		log.Error("Failed to render settings panel", "error", err)
	}
}

// GetSettings returns the current global + per-library settings as JSON.
func (h *AdminHandler) GetSettings(c *gin.Context) {
	app := h.settings.App()
	libs := h.settings.Libraries()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": settingsResponse{
			App:       toAppRequest(app),
			Libraries: toLibraryItems(libs),
		},
	})
}

// cronParser accepts the 5-field cron expressions used elsewhere in the project
// (gocron is configured the same way).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// UpdateAppSettings saves the global settings.
func (h *AdminHandler) UpdateAppSettings(c *gin.Context) {
	var req appSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	schedule := strings.TrimSpace(req.CleanupSchedule)
	if _, err := cronParser.Parse(schedule); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid cron schedule: "+err.Error())
		return
	}
	current := h.settings.App()
	current.DryRun = req.DryRun
	current.CleanupSchedule = schedule
	current.LeavingCollectionsEnabled = req.LeavingCollectionsEnabled
	current.LeavingCollectionsMovieName = strings.TrimSpace(req.LeavingCollectionsMovieName)
	current.LeavingCollectionsTVName = strings.TrimSpace(req.LeavingCollectionsTVName)
	current.CleanupMode = req.CleanupMode
	current.KeepCount = req.KeepCount
	if err := h.settings.SaveApp(c.Request.Context(), current); err != nil {
		log.Error("failed to save app settings", "error", err)
		jsonError(c, http.StatusInternalServerError, "failed to save settings")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": toAppRequest(h.settings.App())})
}

// UpsertLibrarySettings creates or updates one library's settings by name.
func (h *AdminHandler) UpsertLibrarySettings(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		jsonError(c, http.StatusBadRequest, "library name required")
		return
	}
	var req librarySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	existing, _ := h.settings.Library(name)
	ls := database.LibrarySettings{
		Model:                  existing.Model,
		Name:                   name,
		Enabled:                req.Enabled,
		LifetimeDays:           req.LifetimeDays,
		DeletionPeriodDays:     req.DeletionPeriodDays,
		ProtectionDays:         req.ProtectionDays,
		CompletionThresholdPct: req.CompletionThresholdPct,
	}
	if err := h.settings.UpsertLibrary(c.Request.Context(), ls); err != nil {
		log.Error("failed to upsert library settings", "library", name, "error", err)
		jsonError(c, http.StatusInternalServerError, "failed to save library settings")
		return
	}
	saved, _ := h.settings.Library(name)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": toLibraryItem(saved)})
}

// DeleteLibrarySettings removes a library's settings row.
func (h *AdminHandler) DeleteLibrarySettings(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		jsonError(c, http.StatusBadRequest, "library name required")
		return
	}
	if err := h.settings.DeleteLibrary(c.Request.Context(), name); err != nil {
		log.Error("failed to delete library settings", "library", name, "error", err)
		jsonError(c, http.StatusInternalServerError, "failed to delete library settings")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func toAppRequest(a database.AppSettings) appSettingsRequest {
	return appSettingsRequest{
		DryRun:                      a.DryRun,
		CleanupSchedule:             a.CleanupSchedule,
		LeavingCollectionsEnabled:   a.LeavingCollectionsEnabled,
		LeavingCollectionsMovieName: a.LeavingCollectionsMovieName,
		LeavingCollectionsTVName:    a.LeavingCollectionsTVName,
		CleanupMode:                 a.CleanupMode,
		KeepCount:                   a.KeepCount,
	}
}

func toLibraryItem(ls database.LibrarySettings) libraryListItem {
	return libraryListItem{
		Name:                   ls.Name,
		Enabled:                ls.Enabled,
		LifetimeDays:           ls.LifetimeDays,
		DeletionPeriodDays:     ls.DeletionPeriodDays,
		ProtectionDays:         ls.ProtectionDays,
		CompletionThresholdPct: ls.CompletionThresholdPct,
	}
}

func toLibraryItems(rows []database.LibrarySettings) []libraryListItem {
	out := make([]libraryListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, toLibraryItem(r))
	}
	return out
}

// toPageApp / toPageLibraries are thin shims so the templ page receives the
// same struct shapes as the JSON API.
func toPageApp(a database.AppSettings) models.AppSettingsView {
	return models.AppSettingsView{
		DryRun:                      a.DryRun,
		CleanupSchedule:             a.CleanupSchedule,
		LeavingCollectionsEnabled:   a.LeavingCollectionsEnabled,
		LeavingCollectionsMovieName: a.LeavingCollectionsMovieName,
		LeavingCollectionsTVName:    a.LeavingCollectionsTVName,
		CleanupMode:                 a.CleanupMode,
		KeepCount:                   a.KeepCount,
	}
}

func toPageLibraries(rows []database.LibrarySettings) []models.LibrarySettingsView {
	out := make([]models.LibrarySettingsView, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.LibrarySettingsView{
			Name:                   r.Name,
			Enabled:                r.Enabled,
			LifetimeDays:           r.LifetimeDays,
			DeletionPeriodDays:     r.DeletionPeriodDays,
			ProtectionDays:         r.ProtectionDays,
			CompletionThresholdPct: r.CompletionThresholdPct,
		})
	}
	return out
}
