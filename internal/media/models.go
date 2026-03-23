package media

import "time"

// Library represents a configured media library
type Library struct {
    ID                     string      `json:"id"`
    Name                   string      `json:"name"`
    Path                   string      `json:"path"`
    LibraryType            LibraryType `json:"library_type"`
    Enabled                bool        `json:"enabled"`
    RefreshIntervalMinutes int         `json:"refresh_interval_minutes"`
    CreatedAt              time.Time   `json:"created_at"`
    UpdatedAt              time.Time   `json:"updated_at"`
    LastScanAt             *time.Time  `json:"last_scan_at,omitempty"`
}

// LibraryType represents the type of media library
type LibraryType string

const (
    LibraryTypeMovie  LibraryType = "movie"
    LibraryTypeTV     LibraryType = "tv"
    LibraryTypeMusic  LibraryType = "music"
)

// MediaItem represents an indexed media file
type MediaItem struct {
    ID         string    `json:"id"`
    LibraryID  string    `json:"library_id"`
    Path       string    `json:"path"`
    Title      string    `json:"title"`
    Year       int       `json:"year,omitempty"`
    Overview   string    `json:"overview,omitempty"`
    PosterPath string    `json:"poster_path,omitempty"`

    // Video info
    VideoStream *VideoStream `json:"video_stream,omitempty"`

    // Audio tracks
    AudioStreams []AudioStream `json:"audio_streams"`

    // Subtitle tracks
    SubtitleStreams []SubtitleStream `json:"subtitle_streams"`

    // Container & duration
    Container string  `json:"container"`
    Duration  float64 `json:"duration"` // seconds

    // File metadata
    FileSize       int64     `json:"file_size"`
    FileModifiedAt time.Time `json:"file_modified_at"`
    FileHash       string    `json:"file_hash"`

    // Timestamps
    CreatedAt        time.Time  `json:"created_at"`
    UpdatedAt        time.Time  `json:"updated_at"`
    MetadataFetchedAt *time.Time `json:"metadata_fetched_at,omitempty"`
}

// VideoStream represents video track information
type VideoStream struct {
    Index        int     `json:"index"`
    Codec        string  `json:"codec"`
    Profile      string  `json:"profile,omitempty"`
    Level        string  `json:"level,omitempty"`
    Width        int     `json:"width"`
    Height       int     `json:"height"`
    Framerate    float64 `json:"framerate"`
    BitDepth     int     `json:"bit_depth"`
    HDRType      string  `json:"hdr_type,omitempty"` // "hdr10", "hdr10+", "dolby_vision", "hlg"
    IsInterlaced bool    `json:"is_interlaced"`
    Bitrate      int64   `json:"bitrate"`
}

// AudioStream represents an audio track
type AudioStream struct {
    Index     int    `json:"index"`
    Codec     string `json:"codec"`
    Language  string `json:"language,omitempty"`
    Channels  int    `json:"channels"`
    SampleRate int   `json:"sample_rate"`
    Bitrate   int64  `json:"bitrate,omitempty"`
    Title     string `json:"title,omitempty"`
    IsDefault bool   `json:"is_default"`
}

// SubtitleStream represents a subtitle track
type SubtitleStream struct {
    Index        int    `json:"index"`
    Format       string `json:"format"` // "srt", "ass", "ssa", "pgs", "vobsub", "webvtt", "mov_text"
    Language     string `json:"language,omitempty"`
    Title        string `json:"title,omitempty"`
    IsForced     bool   `json:"is_forced"`
    IsExternal   bool   `json:"is_external"`
    ExternalPath string `json:"external_path,omitempty"`
}

// LibraryScanStatus represents the current scan status
type LibraryScanStatus struct {
    LibraryID   string     `json:"library_id"`
    Status      ScanStatus `json:"status"` // "idle", "scanning", "completed", "error"
    TotalFiles  int        `json:"total_files"`
    Processed   int        `json:"processed"`
    CurrentFile string     `json:"current_file,omitempty"`
    StartedAt   *time.Time `json:"started_at,omitempty"`
    Error       string     `json:"error,omitempty"`
}

// ScanStatus represents the scan state
type ScanStatus string

const (
    ScanStatusIdle      ScanStatus = "idle"
    ScanStatusScanning  ScanStatus = "scanning"
    ScanStatusCompleted ScanStatus = "completed"
    ScanStatusError     ScanStatus = "error"
)
