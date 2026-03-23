package stream

import "time"

// Variant represents a quality variant for adaptive streaming
type Variant struct {
    Name      string `json:"name"` // e.g., "1080p", "720p", "480p"
    Width     int    `json:"width"`
    Height    int    `json:"height"`
    Bitrate   int64  `json:"bitrate"`   // bits per second
    AudioBitrate int64 `json:"audio_bitrate"`
}

// HLSOptions configures HLS generation
type HLSOptions struct {
    SegmentDuration int    `json:"segment_duration"` // seconds (default: 6)
    PlaylistSize     int    `json:"playlist_size"`   // number of segments in playlist (default: 0 = infinite)
    StartNumber      int    `json:"start_number"`    // starting segment number (default: 1)
    TempDir          string `json:"temp_dir"`         // temporary directory for segments
    IncludeAudio     bool   `json:"include_audio"`
}

// DASHOptions configures DASH generation
type DASHOptions struct {
    SegmentDuration int    `json:"segment_duration"`
    TempDir          string `json:"temp_dir"`
}

// HLSManifest represents a generated HLS playlist
type HLSManifest struct {
    MasterPlaylist string   `json:"master_playlist"` // path to master.m3u8
    Variants       []VariantPlaylist `json:"variants"`
    TempDir        string   `json:"temp_dir"`
    CreatedAt      time.Time `json:"created_at"`
}

// VariantPlaylist represents a variant playlist
type VariantPlaylist struct {
    Name       string `json:"name"`
    Playlist   string `json:"playlist"`  // path to variant.m3u8
    SegmentsDir string `json:"segments_dir"`
}

// DASHManifest represents a generated DASH manifest
type DASHManifest struct {
    ManifestPath string `json:"manifest_path"` // path to manifest.mpd
    TempDir      string `json:"temp_dir"`
    CreatedAt    time.Time `json:"created_at"`
}

// StreamSession tracks an active streaming session
type StreamSession struct {
    ID           string       `json:"id"`
    MediaItemID  string       `json:"media_item_id"`
    UserID       string       `json:"user_id"`
    DeviceID     string       `json:"device_id"`
    StreamType   StreamType   `json:"stream_type"`   // "hls", "dash", "direct"
    StartTime    time.Time    `json:"start_time"`
    LastAccess   time.Time    `json:"last_access"`
    BytesServed  int64        `json:"bytes_served"`
    ManifestPath string       `json:"manifest_path,omitempty"`
    Variant      string       `json:"variant,omitempty"`
}

// StreamType represents the type of streaming
type StreamType string

const (
    StreamTypeHLS    StreamType = "hls"
    StreamTypeDASH   StreamType = "dash"
    StreamTypeDirect StreamType = "direct"
    StreamTypeRemux  StreamType = "remux"
    StreamTypeTranscode StreamType = "transcode"
)

// DefaultVariants returns the default quality ladder
func DefaultVariants() []Variant {
    return []Variant{
        {Name: "4k", Width: 3840, Height: 2160, Bitrate: 45_000_000, AudioBitrate: 320_000},
        {Name: "1080p", Width: 1920, Height: 1080, Bitrate: 8_000_000, AudioBitrate: 192_000},
        {Name: "720p", Width: 1280, Height: 720, Bitrate: 4_000_000, AudioBitrate: 128_000},
        {Name: "480p", Width: 854, Height: 480, Bitrate: 2_500_000, AudioBitrate: 128_000},
        {Name: "360p", Width: 640, Height: 360, Bitrate: 1_000_000, AudioBitrate: 96_000},
    }
}
