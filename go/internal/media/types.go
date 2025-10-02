package media

import (
	"time"
)

// ServerType represents the type of media server
type ServerType string

const (
	ServerTypeEmby     ServerType = "emby"
	ServerTypePlex     ServerType = "plex"
	ServerTypeJellyfin ServerType = "jellyfin"
)

// ServerConfig holds configuration for a media server
type ServerConfig struct {
	ID          string     `json:"id"`
	Type        ServerType `json:"type"`
	Name        string     `json:"name"`
	BaseURL     string     `json:"base_url"`
	APIKey      string     `json:"api_key"`
	ExternalURL string     `json:"external_url,omitempty"`
	Enabled     bool       `json:"enabled"`
}

// SystemInfo represents server system information
type SystemInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	ServerType ServerType `json:"server_type"`
	Version    string     `json:"version,omitempty"`
}

// User represents a media server user
type User struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	ServerID   string     `json:"server_id"`
	ServerType ServerType `json:"server_type"`
}

// Session represents an active media session (normalized across all server types)
type Session struct {
	// Server identification
	ServerID   string     `json:"server_id"`
	ServerType ServerType `json:"server_type"`

	// Session identification
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`

	// Media information
	ItemID     string `json:"item_id"`
	ItemName   string `json:"item_name"`
	ItemType   string `json:"item_type"`
	PositionMs int64  `json:"position_ms"` // Position in milliseconds (normalized)
	DurationMs int64  `json:"duration_ms"` // Duration in milliseconds (normalized)

	// Client information
	ClientApp     string `json:"client_app"`
	DeviceName    string `json:"device_name"`
	RemoteAddress string `json:"remote_address,omitempty"`

	// Playback details
	PlayMethod string `json:"play_method"` // "Direct", "Transcode", etc.
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
	Container  string `json:"container"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Bitrate    int64  `json:"bitrate"` // Bits per second

	// Audio details
	AudioLanguage string `json:"audio_language,omitempty"`
	AudioChannels int    `json:"audio_channels"`
	AudioDefault  bool   `json:"audio_default"`

	// Subtitle details
	SubtitleLanguage string `json:"subtitle_language,omitempty"`
	SubtitleCodec    string `json:"subtitle_codec,omitempty"`
	SubtitleCount    int    `json:"subtitle_count"`

	// Quality indicators
	DolbyVision bool `json:"dolby_vision"`
	HDR10       bool `json:"hdr10"`

	// Transcode details (when applicable)
	TranscodeContainer  string   `json:"transcode_container,omitempty"`
	TranscodeVideoCodec string   `json:"transcode_video_codec,omitempty"`
	TranscodeAudioCodec string   `json:"transcode_audio_codec,omitempty"`
	TranscodeReasons    []string `json:"transcode_reasons,omitempty"`
	TranscodeProgress   float64  `json:"transcode_progress,omitempty"`
	TranscodeWidth      int      `json:"transcode_width,omitempty"`
	TranscodeHeight     int      `json:"transcode_height,omitempty"`
	TranscodeBitrate    int64    `json:"transcode_bitrate,omitempty"`

	// Track-specific methods
	VideoMethod string `json:"video_method,omitempty"` // "Direct Play", "Transcode"
	AudioMethod string `json:"audio_method,omitempty"` // "Direct Play", "Transcode"

	// State
	IsPaused bool `json:"is_paused"`

	// Timestamps
	LastUpdate time.Time `json:"last_update"`
}

// MediaItem represents a media item with codec information
type MediaItem struct {
	ID             string     `json:"id"`
	ServerID       string     `json:"server_id"`
	ServerType     ServerType `json:"server_type"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	Height         *int       `json:"height,omitempty"`
	Width          *int       `json:"width,omitempty"`
	Codec          string     `json:"video_codec,omitempty"`
	Container      string     `json:"container,omitempty"`
	RuntimeMs      *int64     `json:"runtime_ms,omitempty"`
	BitrateBps     *int64     `json:"bitrate_bps,omitempty"`
	FileSizeBytes  *int64     `json:"file_size_bytes,omitempty"`
	FilePath       string     `json:"file_path,omitempty"` // Physical file path for deduplication
	ProductionYear *int       `json:"production_year,omitempty"`
	Genres         []string   `json:"genres,omitempty"`

	// Episode-specific fields
	SeriesID          string `json:"series_id,omitempty"`
	SeriesName        string `json:"series_name,omitempty"`
	ParentIndexNumber *int   `json:"parent_index_number,omitempty"` // Season
	IndexNumber       *int   `json:"index_number,omitempty"`        // Episode
}

// PlayHistoryItem represents a playback history entry
type PlayHistoryItem struct {
	ID          string     `json:"id"`
	ServerID    string     `json:"server_id"`
	ServerType  ServerType `json:"server_type"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	DatePlayed  string     `json:"date_played"`          // ISO8601
	PlaybackPos int64      `json:"playback_position_ms"` // Position in milliseconds
	UserID      string     `json:"user_id"`
}

// UserDataItem represents a user's playback state for a specific item
type UserDataItem struct {
	ID                 string     `json:"id"`
	ServerID           string     `json:"server_id"`
	ServerType         ServerType `json:"server_type"`
	Name               string     `json:"name"`
	Type               string     `json:"type"`
	RuntimeMs          int64      `json:"runtime_ms"`
	Played             bool       `json:"played"`
	PlayCount          int        `json:"play_count"`
	PlaybackPositionMs int64      `json:"playback_position_ms"`
	LastPlayed         string     `json:"last_played"`
}

// ServerHealth represents the health status of a media server
type ServerHealth struct {
	ServerID     string     `json:"server_id"`
	ServerType   ServerType `json:"server_type"`
	ServerName   string     `json:"server_name"`
	IsReachable  bool       `json:"is_reachable"`
	ResponseTime int64      `json:"response_time_ms"` // Response time in milliseconds
	LastCheck    time.Time  `json:"last_check"`
	Error        string     `json:"error,omitempty"`
}
