package models

import (
	"github.com/jon4hz/jellysweep/internal/database"
)

// ToUserMediaItem converts a database.Media to UserMediaItem for regular users.
// This excludes sensitive fields like RequestedBy. cleanupMode and keepCount are
// surfaced on TV items so the UI can render the correct deletion semantics.
func ToUserMediaItem(m database.Media, cleanupMode string, keepCount int) UserMediaItem {
	item := UserMediaItem{
		ID:              m.ID,
		Title:           m.Title,
		Year:            m.Year,
		MediaType:       MediaType(m.MediaType),
		LibraryName:     m.LibraryName,
		FileSize:        m.FileSize,
		DefaultDeleteAt: m.DefaultDeleteAt,
		Unkeepable:      m.Unkeepable,
	}

	if m.MediaType == database.MediaTypeTV {
		item.CleanupMode = cleanupMode
		item.KeepCount = keepCount
	}

	if m.Request.ID != 0 {
		item.Request = &UserRequestInfo{
			ID:     m.Request.ID,
			Status: string(m.Request.Status),
		}
	}

	return item
}

// ToUserMediaItems converts a slice of database.Media to UserMediaItems.
func ToUserMediaItems(items []database.Media, cleanupMode string, keepCount int) []UserMediaItem {
	result := make([]UserMediaItem, len(items))
	for i, item := range items {
		result[i] = ToUserMediaItem(item, cleanupMode, keepCount)
	}
	return result
}

// ToAdminMediaItem converts a database.Media to AdminMediaItem for admins.
func ToAdminMediaItem(m database.Media, cleanupMode string, keepCount int) AdminMediaItem {
	item := AdminMediaItem{
		ID:              m.ID,
		JellyfinID:      m.JellyfinID,
		LibraryName:     m.LibraryName,
		ArrID:           m.ArrID,
		Title:           m.Title,
		TmdbId:          m.TmdbId,
		TvdbId:          m.TvdbId,
		Year:            m.Year,
		FileSize:        m.FileSize,
		MediaType:       MediaType(m.MediaType),
		RequestedBy:     m.RequestedBy,
		DefaultDeleteAt: m.DefaultDeleteAt,
		ProtectedUntil:  m.ProtectedUntil,
		Unkeepable:      m.Unkeepable,
	}

	if m.MediaType == database.MediaTypeTV {
		item.CleanupMode = cleanupMode
		item.KeepCount = keepCount
	}

	if m.Request.ID != 0 {
		item.Request = &AdminRequestInfo{
			ID:        m.Request.ID,
			UserID:    m.Request.UserID,
			Username:  m.Request.User.Username,
			Status:    string(m.Request.Status),
			CreatedAt: m.Request.CreatedAt,
			UpdatedAt: m.Request.UpdatedAt,
		}
	}

	return item
}

// ToAdminMediaItems converts a slice of database.Media to AdminMediaItems.
func ToAdminMediaItems(items []database.Media, cleanupMode string, keepCount int) []AdminMediaItem {
	result := make([]AdminMediaItem, len(items))
	for i, item := range items {
		result[i] = ToAdminMediaItem(item, cleanupMode, keepCount)
	}
	return result
}

// ToHistoryEventItem converts a database.HistoryEvent to HistoryEventItem.
func ToHistoryEventItem(e database.HistoryEvent) HistoryEventItem {
	username := ""
	if e.User != nil {
		username = e.User.Username
	}

	return HistoryEventItem{
		ID:          e.ID,
		MediaID:     e.MediaID,
		JellyfinID:  e.Media.JellyfinID,
		ArrID:       e.Media.ArrID,
		MediaType:   MediaType(e.Media.MediaType),
		Title:       e.Media.Title,
		Year:        e.Media.Year,
		LibraryName: e.Media.LibraryName,
		EventType:   string(e.EventType),
		Username:    username,
		EventTime:   e.EventTime,
		CreatedAt:   e.CreatedAt,
	}
}

// ToHistoryEventItems converts a slice of database.HistoryEvent to HistoryEventItems.
func ToHistoryEventItems(items []database.HistoryEvent) []HistoryEventItem {
	result := make([]HistoryEventItem, len(items))
	for i, item := range items {
		result[i] = ToHistoryEventItem(item)
	}
	return result
}
