// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxProbeSize is the maximum allowed size for probe JSON in bytes (1MB)
// SECURITY FIX: Prevents memory exhaustion from large JSON payloads
const MaxProbeSize = 1 << 20 // 1MB

// MaxCuratedSize is the maximum allowed size for curated device JSON (5MB)
const MaxCuratedSize = 5 << 20 // 5MB

// MaxFeedbackSize is the maximum allowed size for feedback JSON (100KB)
const MaxFeedbackSize = 100 << 10 // 100KB

// ValidateJSONSize checks if data size is within allowed limits
// SECURITY FIX: Validate size before parsing to prevent memory exhaustion
func ValidateJSONSize(data []byte, maxSize int, dataType string) error {
	if len(data) > maxSize {
		return fmt.Errorf("%s exceeds maximum size: %d bytes (max: %d)", dataType, len(data), maxSize)
	}
	return nil
}

// CodecProbeResult represents the JSON output from a codec probe scenario
type CodecProbeResult struct {
	DeviceID      string                 `json:"device_id"`
	DeviceName    string                 `json:"device_name"`
	Platform      string                 `json:"platform"`
	OSVersion     string                 `json:"os_version"`
	AppVersion    string                 `json:"app_version"`
	Model         string                 `json:"model"`
	Manufacturer  string                 `json:"manufacturer"`
	UserAgent     string                 `json:"user_agent"`
	VideoCodecs   []VideoCodecInfo       `json:"video_codecs"`
	AudioCodecs   []AudioCodecInfo       `json:"audio_codecs"`
	SubtitleFormats []string             `json:"subtitle_formats"`
	ContainerFormats []string            `json:"container_formats"`
	DRMSupport    *DRMSupported          `json:"drm_support"`
	MaxResolution string                 `json:"max_resolution"`
	MaxBitrate    int64                  `json:"max_bitrate"`
	SupportsHDR   bool                   `json:"supports_hdr"`
	SupportsDolbyVision bool             `json:"supports_dolby_vision"`
	SupportsDolbyAtmos  bool             `json:"supports_dolby_atmos"`
	SupportsDTS     bool                 `json:"supports_dts"`
	DirectPlaySupport *DirectPlaySupport `json:"direct_play_support"`
	TranscodingPreferences *TranscodingPrefs `json:"transcoding_preferences"`
	ProbedAt      string                 `json:"probed_at"`
	ProbeDurationMS int                  `json:"probe_duration_ms"`
}

// VideoCodecInfo represents information about a supported video codec
type VideoCodecInfo struct {
	Name        string `json:"name"`
	Profile     string `json:"profile"`
	Level       string `json:"level"`
	BitDepth    int    `json:"bit_depth"`
	MaxWidth    int    `json:"max_width"`
	MaxHeight   int    `json:"max_height"`
	MaxFramerate int   `json:"max_framerate"`
	Supports10bit bool `json:"supports_10bit"`
	SupportsHDR   bool `json:"supports_hdr"`
	SupportsDolbyVision bool `json:"supports_dolby_vision"`
}

// AudioCodecInfo represents information about a supported audio codec
type AudioCodecInfo struct {
	Name        string `json:"name"`
	Channels    int    `json:"channels"`
	SampleRates []int  `json:"sample_rates"`
	Bitrate     int    `json:"bitrate"`
	SupportsSurround bool `json:"supports_surround"`
	SupportsAtmos  bool `json:"supports_atmos"`
	SupportsTrueHD bool `json:"supports_truehd"`
	SupportsDTS    bool `json:"supports_dts"`
}

// DirectPlaySupport represents container and codec support for direct play
type DirectPlaySupport struct {
	VideoContainers   []string `json:"video_containers"`
	AudioContainers   []string `json:"audio_containers"`
	SubtitleFormats   []string `json:"subtitle_formats"`
	MaxVideoBitrate   int64    `json:"max_video_bitrate"`
	MaxAudioBitrate   int64    `json:"max_audio_bitrate"`
}

// TranscodingPrefs represents preferred transcoding settings
type TranscodingPrefs struct {
	PreferredVideoCodec   string `json:"preferred_video_codec"`
	PreferredAudioCodec   string `json:"preferred_audio_codec"`
	PreferredContainer    string `json:"preferred_container"`
	MaxTranscodeBitrate   int64  `json:"max_transcode_bitrate"`
	PreferHardwareEncoding bool  `json:"prefer_hardware_encoding"`
}

// ParseCodecProbe parses a codec probe JSON result into DeviceCapabilities
// SECURITY FIX: Validates JSON size before parsing to prevent memory exhaustion
func ParseCodecProbe(probeJSON []byte) (*DeviceCapabilities, error) {
	// Validate size before parsing
	if err := ValidateJSONSize(probeJSON, MaxProbeSize, "probe JSON"); err != nil {
		return nil, err
	}

	var probe CodecProbeResult
	if err := json.Unmarshal(probeJSON, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse probe JSON: %w", err)
	}

	return ParseCodecProbeResult(&probe)
}

// ParseCodecProbeResult parses a CodecProbeResult into DeviceCapabilities
func ParseCodecProbeResult(probe *CodecProbeResult) (*DeviceCapabilities, error) {
	if probe == nil {
		return nil, fmt.Errorf("probe result is nil")
	}

	caps := &DeviceCapabilities{
		DeviceID:        probe.DeviceID,
		DeviceName:      probe.DeviceName,
		Platform:        probe.Platform,
		OSVersion:       probe.OSVersion,
		AppVersion:      probe.AppVersion,
		Model:           probe.Model,
		Manufacturer:    probe.Manufacturer,
		UserAgent:       probe.UserAgent,
		DRMSupport:      probe.DRMSupport,
		MaxBitrate:      probe.MaxBitrate,
		SupportsHDR:     probe.SupportsHDR,
		SupportsDolbyVision: probe.SupportsDolbyVision,
		SupportsDolbyAtmos:  probe.SupportsDolbyAtmos,
		SupportsDTS:     probe.SupportsDTS,
		DirectPlaySupport: probe.DirectPlaySupport,
		TranscodingPreferences: probe.TranscodingPreferences,
	}

	// Parse video codecs
	videoCodecs := make(map[string]bool)
	maxWidth := 0
	maxHeight := 0
	supports10Bit := false

	for _, vc := range probe.VideoCodecs {
		codecName := strings.ToLower(vc.Name)
		videoCodecs[codecName] = true

		if vc.MaxWidth > maxWidth {
			maxWidth = vc.MaxWidth
		}
		if vc.MaxHeight > maxHeight {
			maxHeight = vc.MaxHeight
		}
		if vc.Supports10bit || vc.BitDepth >= 10 {
			supports10Bit = true
		}
	}

	caps.VideoCodecs = make([]string, 0, len(videoCodecs))
	for codec := range videoCodecs {
		caps.VideoCodecs = append(caps.VideoCodecs, codec)
	}
	caps.Supports10Bit = supports10Bit

	// Parse audio codecs
	audioCodecs := make(map[string]bool)
	for _, ac := range probe.AudioCodecs {
		audioCodecs[strings.ToLower(ac.Name)] = true
	}

	caps.AudioCodecs = make([]string, 0, len(audioCodecs))
	for codec := range audioCodecs {
		caps.AudioCodecs = append(caps.AudioCodecs, codec)
	}

	// Parse subtitle formats
	caps.SubtitleFormats = probe.SubtitleFormats

	// Parse container formats
	caps.ContainerFormats = probe.ContainerFormats

	// Parse resolution
	if probe.MaxResolution != "" {
		width, height, err := parseResolution(probe.MaxResolution)
		if err == nil {
			if width > maxWidth {
				maxWidth = width
			}
			if height > maxHeight {
				maxHeight = height
			}
		}
	}

	caps.MaxWidth = maxWidth
	caps.MaxHeight = maxHeight

	// Parse timestamp
	if probe.ProbedAt != "" {
		if t, err := time.Parse(time.RFC3339, probe.ProbedAt); err == nil {
			caps.ProbedAt = t
		} else if t, err := time.Parse(time.RFC3339Nano, probe.ProbedAt); err == nil {
			caps.ProbedAt = t
		}
	} else {
		caps.ProbedAt = time.Now()
	}

	caps.ProbeDurationMS = probe.ProbeDurationMS

	return caps, nil
}

// parseResolution parses a resolution string like "1920x1080" into width and height
func parseResolution(res string) (int, int, error) {
	parts := strings.Split(res, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid resolution format: %s", res)
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %w", err)
	}

	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %w", err)
	}

	return width, height, nil
}

// ToJSON serializes DeviceCapabilities to JSON
func (d *DeviceCapabilities) ToJSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// ToMap converts DeviceCapabilities to a map for database storage
func (d *DeviceCapabilities) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"device_id":           d.DeviceID,
		"device_name":         d.DeviceName,
		"platform":            d.Platform,
		"os_version":          d.OSVersion,
		"app_version":         d.AppVersion,
		"model":               d.Model,
		"manufacturer":        d.Manufacturer,
		"user_agent":          d.UserAgent,
		"video_codecs":        d.VideoCodecs,
		"audio_codecs":        d.AudioCodecs,
		"subtitle_formats":    d.SubtitleFormats,
		"container_formats":   d.ContainerFormats,
		"drm_support":         d.DRMSupport,
		"max_width":           d.MaxWidth,
		"max_height":          d.MaxHeight,
		"max_bitrate":         d.MaxBitrate,
		"supports_hdr":        d.SupportsHDR,
		"supports_dolby_vision": d.SupportsDolbyVision,
		"supports_dolby_atmos":  d.SupportsDolbyAtmos,
		"supports_dts":        d.SupportsDTS,
		"supports_10bit":      d.Supports10Bit,
		"direct_play_support":      d.DirectPlaySupport,
		"transcoding_preferences":  d.TranscodingPreferences,
		"probed_at":                d.ProbedAt,
		"probe_duration_ms":        d.ProbeDurationMS,
	}
}