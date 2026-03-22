// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

// TrustLevel represents the trust level of a device
type TrustLevel int

const (
	// TrustLevelUnknown - Device not yet evaluated
	TrustLevelUnknown TrustLevel = iota
	// TrustLevelUntrusted - Device marked as untrusted
	TrustLevelUntrusted
	// TrustLevelLow - Low trust, limited capabilities assumed
	TrustLevelLow
	// TrustLevelMedium - Medium trust, standard capabilities
	TrustLevelMedium
	// TrustLevelHigh - High trust, full capabilities
	TrustLevelHigh
	// TrustLevelTrusted - Explicitly trusted device
	TrustLevelTrusted
)

// String returns the string representation of TrustLevel
func (t TrustLevel) String() string {
	switch t {
	case TrustLevelUnknown:
		return "unknown"
	case TrustLevelUntrusted:
		return "untrusted"
	case TrustLevelLow:
		return "low"
	case TrustLevelMedium:
		return "medium"
	case TrustLevelHigh:
		return "high"
	case TrustLevelTrusted:
		return "trusted"
	default:
		return "unknown"
	}
}

// ParseTrustLevel parses a string into TrustLevel
func ParseTrustLevel(s string) TrustLevel {
	switch s {
	case "unknown":
		return TrustLevelUnknown
	case "untrusted":
		return TrustLevelUntrusted
	case "low":
		return TrustLevelLow
	case "medium":
		return TrustLevelMedium
	case "high":
		return TrustLevelHigh
	case "trusted":
		return TrustLevelTrusted
	default:
		return TrustLevelUnknown
	}
}

// DevicePlatform represents the platform type of a device
type DevicePlatform struct {
	Type       string  `json:"type"`        // e.g., "smart_tv", "mobile", "desktop", "gaming_console", "streaming_box"
	Name       string  `json:"name"`        // e.g., "Roku", "Apple TV", "Android TV"
	OS         string  `json:"os"`          // Operating system name
	OSVersion  string  `json:"os_version"`  // Operating system version
	WebBrowser string  `json:"web_browser"` // Browser if applicable
	BrowserVer string  `json:"browser_ver"` // Browser version if applicable
}

// DeviceIdentity identifies a device
type DeviceIdentity struct {
	DeviceID    string           `json:"device_id"`
	Name        string           `json:"name"`
	Model       string           `json:"model"`
	Manufacturer string          `json:"manufacturer"`
	Platform    DevicePlatform   `json:"platform"`
	IPAddress   string           `json:"ip_address,omitempty"`
	UserAgent   string           `json:"user_agent,omitempty"`
}

// TrustedCodecSupport represents codec support from trusted sources
type TrustedCodecSupport struct {
	VideoCodecs   []string `json:"video_codecs"`
	AudioCodecs   []string `json:"audio_codecs"`
	SubtitleFormats []string `json:"subtitle_formats"`
	ContainerFormats []string `json:"container_formats"`
	MaxWidth      int      `json:"max_width"`
	MaxHeight     int      `json:"max_height"`
	MaxBitrate    int64    `json:"max_bitrate"`
	SupportsHDR   bool     `json:"supports_hdr"`
	SupportsDV    bool     `json:"supports_dolby_vision"`
	SupportsAtmos bool     `json:"supports_dolby_atmos"`
	SupportsDTS   bool     `json:"supports_dts"`
	Source        string   `json:"source"` // "curated", "official", "community"
}

// ScenarioSupport represents support for specific probe scenarios
type ScenarioSupport struct {
	ScenarioID   string `json:"scenario_id"`
	Supported    bool   `json:"supported"`
	DirectPlay   bool   `json:"direct_play"`
	TranscodeVideo bool `json:"transcode_video"`
	TranscodeAudio bool `json:"transcode_audio"`
	TranscodeAll bool   `json:"transcode_all"`
	MaxBitrate   int64  `json:"max_bitrate,omitempty"`
	VideoCodec   string `json:"video_codec,omitempty"`
	AudioCodec   string `json:"audio_codec,omitempty"`
	Container    string `json:"container,omitempty"`
}

// DeviceCapabilities represents the capabilities of a device
type DeviceCapabilities struct {
	Identity          DeviceIdentity         `json:"identity"`
	TrustedSupport    *TrustedCodecSupport   `json:"trusted_support,omitempty"`
	ScenarioSupport   []ScenarioSupport      `json:"scenario_support"`
	VideoCodecs       []string               `json:"video_codecs"`
	AudioCodecs       []string               `json:"audio_codecs"`
	SubtitleFormats   []string               `json:"subtitle_formats"`
	ContainerFormats  []string               `json:"container_formats"`
	MaxWidth          int                    `json:"max_width"`
	MaxHeight         int                    `json:"max_height"`
	MaxBitrate        int64                  `json:"max_bitrate"`
	SupportsHDR       bool                   `json:"supports_hdr"`
	SupportsDV        bool                   `json:"supports_dolby_vision"`
	SupportsAtmos     bool                   `json:"supports_dolby_atmos"`
	SupportsDTS       bool                   `json:"supports_dts"`
	DirectPlaySupport map[string]bool        `json:"direct_play_support"`
	TranscodePrefs    map[string]interface{} `json:"transcode_preferences"`
	TrustLevel        TrustLevel             `json:"trust_level"`
	TrustScore        float64                `json:"trust_score"`
	LastUpdated       string                 `json:"last_updated"`
}

// ProbeReport represents a probe result report
type ProbeReport struct {
	DeviceCapabilities DeviceCapabilities     `json:"device_capabilities"`
	ScenariosRun       []ScenarioResult       `json:"scenarios_run"`
	Errors             []string               `json:"errors,omitempty"`
	Warnings           []string               `json:"warnings,omitempty"`
	DurationMs         int64                  `json:"duration_ms"`
	Timestamp          string                 `json:"timestamp"`
}

// ScenarioResult represents the result of a single probe scenario
type ScenarioResult struct {
	ScenarioID   string `json:"scenario_id"`
	Passed       bool   `json:"passed"`
	DirectPlay   bool   `json:"direct_play"`
	Bitrate      int64  `json:"bitrate"`
	DurationMs   int64  `json:"duration_ms"`
	Errors       []string `json:"errors,omitempty"`
}

// Common codec constants
var (
	VideoCodecs = []string{"h264", "hevc", "vp9", "av1", "mpeg2", "mpeg4", "vc1", "vp8"}
	AudioCodecs = []string{"aac", "mp3", "flac", "alac", "opus", "ac3", "eac3", "dts", "truehd", "aac_low", "aac_he", "aac_he_v2"}
	SubtitleFormats = []string{"srt", "vtt", "ssa", "ass", "sub", "pgs", "dvdsub"}
	ContainerFormats = []string{"mp4", "mkv", "ts", "m2ts", "mov", "avi", "webm", "flv"}
)

// Common platform types
const (
	PlatformSmartTV        = "smart_tv"
	PlatformMobile         = "mobile"
	PlatformDesktop        = "desktop"
	PlatformGamingConsole  = "gaming_console"
	PlatformStreamingBox   = "streaming_box"
	PlatformWebBrowser     = "web_browser"
	PlatformRoku           = "roku"
	PlatformAppleTV        = "apple_tv"
	PlatformAndroidTV      = "android_tv"
	PlatformFireTV         = "fire_tv"
	PlatformChromecast     = "chromecast"
	PlatformPlayStation    = "playstation"
	PlatformXbox           = "xbox"
	PlatformNintendoSwitch = "nintendo_switch"
)
