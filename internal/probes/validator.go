// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tenkile/tenkile/pkg/codec"
)

// Platform detection patterns - compiled once at package initialization
var platformPatterns = map[string]*regexp.Regexp{
	"ios":        regexp.MustCompile(`(?i)(iphone|ipad|ipod)`),
	"android":    regexp.MustCompile(`(?i)android`),
	"macos":      regexp.MustCompile(`(?i)(mac os|macintosh)`),
	"windows":    regexp.MustCompile(`(?i)windows`),
	"linux":      regexp.MustCompile(`(?i)linux`),
	"chromecast": regexp.MustCompile(`(?i)(crkey|chromecast)`),
	"roku":       regexp.MustCompile(`(?i)roku`),
	"firetv":     regexp.MustCompile(`(?i)(fire tv|firetv)`),
	"xbox":       regexp.MustCompile(`(?i)xbox`),
	"playstation": regexp.MustCompile(`(?i)playstation`),
}

// ValidationResult represents the result of validating probe data
type ValidationResult struct {
	IsValid    bool              `json:"is_valid"`
	Score      float64           `json:"score"`       // 0.0 to 1.0
	Errors     []ValidationError `json:"errors"`
	Warnings   []ValidationWarning `json:"warnings"`
	AnomalyDetected bool         `json:"anomaly_detected"`
	AnomalyReason string         `json:"anomaly_reason,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Severity string `json:"severity"` // "critical", "error", "warning"
	Field   string `json:"field,omitempty"`
}

// ValidationWarning represents a non-critical validation issue
type ValidationWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// AnomalyRecord tracks detected anomalies for a device
type AnomalyRecord struct {
	DeviceID    string    `json:"device_id"`
	AnomalyType string    `json:"anomaly_type"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Count       int       `json:"count"`
}

// Validator validates probe data and detects anomalies
type Validator struct {
	// Known platform constraints
	PlatformConstraints map[string]*PlatformConstraints

	// Anomaly tracking
	anomalyHistory map[string][]AnomalyRecord

	// Configuration
	StrictMode     bool
	MaxAnomalies   int
	AnomalyWindow  time.Duration
}

// PlatformConstraints defines constraints for a specific platform
type PlatformConstraints struct {
	AllowedVideoCodecs   []string
	AllowedAudioCodecs   []string
	AllowedContainers    []string
	MaxVideoCodecs       int
	MaxAudioCodecs       int
	MinVideoCodecs       int
	RequiresH264         bool
	RequiresAAC          bool
	MaxBitrate           int64
	SupportedPlatforms   []string
}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	v := &Validator{
		PlatformConstraints: make(map[string]*PlatformConstraints),
		anomalyHistory:      make(map[string][]AnomalyRecord),
		StrictMode:          false,
		MaxAnomalies:        10,
		AnomalyWindow:       24 * time.Hour,
	}

	v.initPlatformConstraints()
	return v
}

// initPlatformConstraints initializes known platform constraints
func (v *Validator) initPlatformConstraints() {
	// iOS/macOS constraints
	v.PlatformConstraints["ios"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "mpeg4", "mpeg2", "vc1"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "alac", "ac3", "eac3", "truehd"},
		AllowedContainers:  []string{"mp4", "mov", "mkv", "ts"},
		MaxVideoCodecs:     8,
		MaxAudioCodecs:     10,
		MinVideoCodecs:     2,
		RequiresH264:       true,
		MaxBitrate:         100000000, // 100 Mbps max
	}

	v.PlatformConstraints["tvos"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "alac", "ac3", "eac3", "truehd", "opus"},
		AllowedContainers:  []string{"mp4", "mov", "mkv", "webm"},
		MaxVideoCodecs:     10,
		MaxAudioCodecs:     12,
		RequiresH264:       true,
		MaxBitrate:         100000000,
	}

	v.PlatformConstraints["android"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1", "vp8", "mpeg4", "mpeg2"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "opus", "ac3", "eac3", "dts"},
		AllowedContainers:  []string{"mp4", "mkv", "webm", "3gp"},
		MaxVideoCodecs:     12,
		MaxAudioCodecs:     12,
		MinVideoCodecs:     2,
		RequiresH264:       true,
		MaxBitrate:         100000000,
	}

	v.PlatformConstraints["web"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1", "vp8"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "opus", "ac3", "eac3"},
		AllowedContainers:  []string{"mp4", "mkv", "webm", "ogg"},
		MaxVideoCodecs:     8,
		MaxAudioCodecs:     8,
		RequiresH264:       true,
		MaxBitrate:         50000000,
	}

	v.PlatformConstraints["chromecast"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "opus", "ac3", "eac3"},
		AllowedContainers:  []string{"mp4", "mkv", "webm"},
		MaxVideoCodecs:     6,
		RequiresH264:       true,
		MaxBitrate:         25000000,
	}

	v.PlatformConstraints["roku"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "mpeg4", "mpeg2", "vp8"},
		AllowedAudioCodecs: []string{"aac", "mp3", "ac3", "eac3", "flac"},
		AllowedContainers:  []string{"mp4", "mkv", "mov", "ts"},
		MaxVideoCodecs:     8,
		MaxAudioCodecs:     8,
		RequiresH264:       true,
		MaxBitrate:         30000000,
	}

	v.PlatformConstraints["firetv"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1", "mpeg4"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "opus", "ac3", "eac3", "dts"},
		AllowedContainers:  []string{"mp4", "mkv", "webm"},
		MaxVideoCodecs:     8,
		MaxAudioCodecs:     10,
		RequiresH264:       true,
		MaxBitrate:         40000000,
	}

	v.PlatformConstraints["xbox"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "mpeg2", "mpeg4", "vc1"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "alac", "ac3", "eac3", "truehd", "dts"},
		AllowedContainers:  []string{"mp4", "mkv", "mov", "avi", "ts"},
		MaxVideoCodecs:     10,
		MaxAudioCodecs:     12,
		RequiresH264:       true,
		MaxBitrate:         100000000,
	}

	v.PlatformConstraints["playstation"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "mpeg2", "mpeg4"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "ac3", "eac3", "dts", "truehd"},
		AllowedContainers:  []string{"mp4", "mkv", "avi", "ts"},
		MaxVideoCodecs:     6,
		MaxAudioCodecs:     8,
		RequiresH264:       true,
		MaxBitrate:         80000000,
	}

	v.PlatformConstraints["apple_tv"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "alac", "ac3", "eac3", "truehd"},
		AllowedContainers:  []string{"mp4", "mov", "mkv"},
		MaxVideoCodecs:     6,
		MaxAudioCodecs:     8,
		RequiresH264:       true,
		MaxBitrate:         100000000,
	}

	v.PlatformConstraints["windows"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1", "vp8", "mpeg2", "mpeg4", "vc1"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "alac", "opus", "ac3", "eac3", "dts", "truehd"},
		AllowedContainers:  []string{"mp4", "mkv", "avi", "mov", "ts", "wmv", "flv"},
		MaxVideoCodecs:     15,
		MaxAudioCodecs:     15,
		MaxBitrate:         100000000,
	}

	v.PlatformConstraints["macos"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1", "mpeg2", "mpeg4"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "alac", "opus", "ac3", "eac3"},
		AllowedContainers:  []string{"mp4", "mov", "mkv", "webm", "avi"},
		MaxVideoCodecs:     10,
		MaxAudioCodecs:     10,
		RequiresH264:       true,
		MaxBitrate:         100000000,
	}

	v.PlatformConstraints["linux"] = &PlatformConstraints{
		AllowedVideoCodecs: []string{"h264", "hevc", "vp9", "av1", "vp8", "mpeg2", "mpeg4"},
		AllowedAudioCodecs: []string{"aac", "mp3", "flac", "opus", "ac3", "eac3", "dts"},
		AllowedContainers:  []string{"mp4", "mkv", "webm", "avi", "ts", "ogv"},
		MaxVideoCodecs:     12,
		MaxAudioCodecs:     12,
		MaxBitrate:         100000000,
	}
}

// ValidateCapabilities validates device capabilities and returns a ValidationResult
func (v *Validator) ValidateCapabilities(caps *DeviceCapabilities) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		Score:     1.0,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
	}

	if caps == nil {
		result.IsValid = false
		result.Score = 0.0
		result.Errors = append(result.Errors, ValidationError{
			Code:    "EMPTY_DATA",
			Message: "No capabilities data provided",
			Severity: "critical",
		})
		return result
	}

	// Validate platform
	v.validatePlatform(caps, result)

	// Validate codecs
	v.validateVideoCodecs(caps, result)
	v.validateAudioCodecs(caps, result)

	// Validate containers
	v.validateContainers(caps, result)

	// Validate resolution and bitrate
	v.validateResolution(caps, result)
	v.validateBitrate(caps, result)

	// Validate feature flags consistency
	v.validateFeatureFlags(caps, result)

	// Validate DRM consistency
	v.validateDRMConsistency(caps, result)

	// Check for anomalies
	v.checkAnomalies(caps, result)

	// Calculate final score
	result.Score = v.calculateScore(result)

	// Determine overall validity
	if len(result.Errors) > 0 {
		result.IsValid = false
	}

	return result
}

// validatePlatform checks platform validity
func (v *Validator) validatePlatform(caps *DeviceCapabilities, result *ValidationResult) {
	if caps.Platform == "" {
		result.Errors = append(result.Errors, ValidationError{
			Code: "MISSING_PLATFORM",
			Message: "Platform is required",
			Severity: "critical",
			Field: "platform",
		})
		return
	}

	// Normalize platform name
	platform := strings.ToLower(caps.Platform)
	platform = strings.ReplaceAll(platform, " ", "_")
	platform = strings.ReplaceAll(platform, "-", "_")

	_, ok := v.PlatformConstraints[platform]
	if !ok {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "UNKNOWN_PLATFORM",
			Message: fmt.Sprintf("Unknown platform: %s", caps.Platform),
			Field: "platform",
		})
		result.Score -= 0.1
	}

	// Check user agent consistency
	if caps.UserAgent != "" {
		v.checkUserAgentConsistency(platform, caps.UserAgent, result)
	}
}

// validateVideoCodecs checks video codec validity
func (v *Validator) validateVideoCodecs(caps *DeviceCapabilities, result *ValidationResult) {
	constraints, ok := v.PlatformConstraints[caps.Platform]
	if !ok {
		return // Skip codec validation for unknown platforms
	}

	if len(caps.VideoCodecs) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Code: "NO_VIDEO_CODECS",
			Message: "No video codecs reported",
			Severity: "error",
			Field: "video_codecs",
		})
		return
	}

	// Check minimum video codecs
	if constraints.MinVideoCodecs > 0 && len(caps.VideoCodecs) < constraints.MinVideoCodecs {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "LOW_VIDEO_CODECS",
			Message: fmt.Sprintf("Fewer video codecs than expected: %d (expected >= %d)",
				len(caps.VideoCodecs), constraints.MinVideoCodecs),
			Field: "video_codecs",
		})
	}

	// Check maximum video codecs
	if len(caps.VideoCodecs) > constraints.MaxVideoCodecs {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "EXCESSIVE_VIDEO_CODECS",
			Message: fmt.Sprintf("Unusually high number of video codecs: %d (max: %d)",
				len(caps.VideoCodecs), constraints.MaxVideoCodecs),
			Field: "video_codecs",
		})
		result.AnomalyDetected = true
		result.AnomalyReason = "Excessive video codec count"
	}

	// Check for unknown codecs
	for _, vc := range caps.VideoCodecs {
		if !codec.IsVideoCodec(vc) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "UNKNOWN_VIDEO_CODEC",
				Message: fmt.Sprintf("Unknown or invalid video codec: %s", vc),
				Field: "video_codecs",
			})
		}
	}

	// Check for required codecs
	if constraints.RequiresH264 {
		hasH264 := false
		for _, vc := range caps.VideoCodecs {
			if vc == "h264" {
				hasH264 = true
				break
			}
		}
		if !hasH264 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "MISSING_H264",
				Message: "H.264 codec not reported (usually required)",
				Field: "video_codecs",
			})
		}
	}
}

// validateAudioCodecs checks audio codec validity
func (v *Validator) validateAudioCodecs(caps *DeviceCapabilities, result *ValidationResult) {
	if len(caps.AudioCodecs) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "NO_AUDIO_CODECS",
			Message: "No audio codecs reported",
			Field: "audio_codecs",
		})
		return
	}

	// Check for common audio codecs
	hasCommonCodec := false
	for _, ac := range caps.AudioCodecs {
		if ac == "aac" || ac == "mp3" {
			hasCommonCodec = true
			break
		}
	}

	if !hasCommonCodec {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "MISSING_COMMON_AUDIO",
			Message: "No common audio codecs (AAC/MP3) reported",
			Field: "audio_codecs",
		})
	}

	// Check for impossible combinations
	hasDolbyAtmos := false

	for _, ac := range caps.AudioCodecs {
		if ac == "ac3" || ac == "eac3" {
			hasDolbyAtmos = hasDolbyAtmos || caps.SupportsDolbyAtmos
		}
	}

	// Dolby Atmos without AC3/EAC3 is suspicious
	if caps.SupportsDolbyAtmos && !hasDolbyAtmos {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "ATMOS_WITHOUT_BASE",
			Message: "Dolby Atmos reported without AC3/EAC3 support",
			Field: "audio_codecs",
		})
	}
}

// validateContainers checks container format validity
func (v *Validator) validateContainers(caps *DeviceCapabilities, result *ValidationResult) {
	if len(caps.ContainerFormats) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "NO_CONTAINERS",
			Message: "No container formats reported",
			Field: "container_formats",
		})
		return
	}

	// Check for common containers
	hasCommonContainer := false
	for _, cf := range caps.ContainerFormats {
		cf = strings.ToLower(cf)
		if cf == "mp4" || cf == "mkv" {
			hasCommonContainer = true
			break
		}
	}

	if !hasCommonContainer {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "MISSING_COMMON_CONTAINERS",
			Message: "No common container formats (MP4/MKV) reported",
			Field: "container_formats",
		})
	}
}

// validateResolution checks resolution validity
func (v *Validator) validateResolution(caps *DeviceCapabilities, result *ValidationResult) {
	if caps.MaxWidth <= 0 && caps.MaxHeight <= 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "NO_RESOLUTION",
			Message: "No maximum resolution reported",
			Field: "max_resolution",
		})
		return
	}

	// Check for unrealistic resolutions
	if caps.MaxWidth > 16384 || caps.MaxHeight > 16384 {
		result.Errors = append(result.Errors, ValidationError{
			Code: "IMPOSSIBLE_RESOLUTION",
			Message: fmt.Sprintf("Unrealistic resolution: %dx%d", caps.MaxWidth, caps.MaxHeight),
			Severity: "error",
			Field: "max_resolution",
		})
	}

	// Check for standard aspect ratios
	if caps.MaxWidth > 0 && caps.MaxHeight > 0 {
		ratio := float64(caps.MaxWidth) / float64(caps.MaxHeight)
		standardRatios := []float64{4.0/3.0, 16.0/9.0, 21.0/9.0, 1.0}
		isStandard := false
		for _, sr := range standardRatios {
			if abs(ratio-sr) < 0.05 {
				isStandard = true
				break
			}
		}
		if !isStandard {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "UNUSUAL_ASPECT_RATIO",
				Message: fmt.Sprintf("Unusual aspect ratio: %.2f", ratio),
				Field: "max_resolution",
			})
		}
	}
}

// validateBitrate checks bitrate validity
func (v *Validator) validateBitrate(caps *DeviceCapabilities, result *ValidationResult) {
	if caps.MaxBitrate <= 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "NO_BITRATE",
			Message: "No maximum bitrate reported",
			Field: "max_bitrate",
		})
		return
	}

	// Check for unrealistic bitrates
	if caps.MaxBitrate > 1000000000 { // 1 Gbps
		result.Errors = append(result.Errors, ValidationError{
			Code: "IMPOSSIBLE_BITRATE",
			Message: fmt.Sprintf("Unrealistic bitrate: %d bps", caps.MaxBitrate),
			Severity: "error",
			Field: "max_bitrate",
		})
	}
}

// validateFeatureFlags checks feature flag consistency
func (v *Validator) validateFeatureFlags(caps *DeviceCapabilities, result *ValidationResult) {
	// HDR without 10-bit is suspicious
	if caps.SupportsHDR && !caps.Supports10Bit {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "HDR_WITHOUT_10BIT",
			Message: "HDR support reported without 10-bit support",
			Field: "supports_hdr",
		})
	}

	// Dolby Vision requires specific codec support
	if caps.SupportsDolbyVision {
		hasHEVC := false
		hasH264 := false
		for _, vc := range caps.VideoCodecs {
			if vc == "hevc" {
				hasHEVC = true
			}
			if vc == "h264" {
				hasH264 = true
			}
		}
		if !hasHEVC && !hasH264 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "DOLBY_VISION_WITHOUT_CODEC",
				Message: "Dolby Vision reported without HEVC or H.264 support",
				Field: "supports_dolby_vision",
			})
		}
	}

	// Dolby Atmos requires AC3/EAC3
	if caps.SupportsDolbyAtmos {
		hasAC3 := false
		for _, ac := range caps.AudioCodecs {
			if ac == "ac3" || ac == "eac3" {
				hasAC3 = true
				break
			}
		}
		if !hasAC3 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "ATMOS_WITHOUT_AC3",
				Message: "Dolby Atmos reported without AC3/EAC3 support",
				Field: "supports_dolby_atmos",
			})
		}
	}

	// DTS flag requires DTS codec
	if caps.SupportsDTS {
		hasDTS := false
		for _, ac := range caps.AudioCodecs {
			if ac == "dts" {
				hasDTS = true
				break
			}
		}
		if !hasDTS {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "DTS_FLAG_WITHOUT_CODEC",
				Message: "DTS support flag set without DTS codec",
				Field: "supports_dts",
			})
		}
	}
}

// validateDRMConsistency checks DRM support consistency
func (v *Validator) validateDRMConsistency(caps *DeviceCapabilities, result *ValidationResult) {
	if caps.DRMSupport == nil {
		return
	}

	// Check for platform-specific DRM expectations
	switch strings.ToLower(caps.Platform) {
	case "ios", "tvos", "apple_tv", "macos":
		// Apple platforms should support FairPlay
		if !IsDRMSupported(caps.DRMSupport, "fairplay") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "APPLE_WITHOUT_FAIRPLAY",
				Message: "Apple platform without FairPlay DRM support",
				Field: "drm_support",
			})
		}
	case "android", "chromecast", "firetv":
		// Android platforms should support Widevine
		if !IsDRMSupported(caps.DRMSupport, "widevine") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "ANDROID_WITHOUT_WIDEVINE",
				Message: "Android platform without Widevine DRM support",
				Field: "drm_support",
			})
		}
	case "xbox", "windows":
		// Windows platforms should support PlayReady
		if !IsDRMSupported(caps.DRMSupport, "playready") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Code: "WINDOWS_WITHOUT_PLAYREADY",
				Message: "Windows platform without PlayReady DRM support",
				Field: "drm_support",
			})
		}
	}
}

// checkUserAgentConsistency checks if user agent matches platform
func (v *Validator) checkUserAgentConsistency(platform, userAgent string, result *ValidationResult) {
	userAgentLower := strings.ToLower(userAgent)

	pattern, ok := platformPatterns[platform]
	if !ok {
		return
	}

	if !pattern.MatchString(userAgentLower) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Code: "UA_PLATFORM_MISMATCH",
			Message: fmt.Sprintf("User agent doesn't match platform: %s", platform),
			Field: "user_agent",
		})
	}
}

// checkAnomalies checks for detected anomalies
func (v *Validator) checkAnomalies(caps *DeviceCapabilities, result *ValidationResult) {
	// Check for capability changes
	history := v.anomalyHistory[caps.DeviceID]

	// Record anomalies from this validation
	for _, err := range result.Errors {
		if err.Severity == "critical" || err.Severity == "error" {
			v.recordAnomaly(caps.DeviceID, err.Code, err.Message, err.Severity)
		}
	}

	for _, warn := range result.Warnings {
		if warn.Code == "EXCESSIVE_VIDEO_CODECS" || warn.Code == "UA_PLATFORM_MISMATCH" {
			v.recordAnomaly(caps.DeviceID, warn.Code, warn.Message, "warning")
		}
	}

	// Check anomaly count
	anomalyCount := 0
	now := time.Now()
	for _, record := range history {
		if now.Sub(record.FirstSeen) < v.AnomalyWindow {
			anomalyCount += record.Count
		}
	}

	if anomalyCount > v.MaxAnomalies {
		result.AnomalyDetected = true
		result.AnomalyReason = fmt.Sprintf("High anomaly count: %d in %v", anomalyCount, v.AnomalyWindow)
		result.Score -= 0.2
	}
}

// recordAnomaly records an anomaly for a device
func (v *Validator) recordAnomaly(deviceID, anomalyType, description, severity string) {
	history := v.anomalyHistory[deviceID]

	// Find existing record
	for i := range history {
		if history[i].AnomalyType == anomalyType {
			history[i].Count++
			history[i].LastSeen = time.Now()
			v.anomalyHistory[deviceID] = history
			return
		}
	}

	// Create new record
	newRecord := AnomalyRecord{
		DeviceID:    deviceID,
		AnomalyType: anomalyType,
		Description: description,
		Severity:    severity,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Count:       1,
	}

	v.anomalyHistory[deviceID] = append(history, newRecord)
}

// calculateScore calculates the final validation score
func (v *Validator) calculateScore(result *ValidationResult) float64 {
	score := 1.0

	// Deduct for errors
	for _, err := range result.Errors {
		switch err.Severity {
		case "critical":
			score -= 0.3
		case "error":
			score -= 0.15
		case "warning":
			score -= 0.05
		}
	}

	// Deduct for warnings
	for range result.Warnings {
		score -= 0.02
	}

	// Deduct for anomalies
	if result.AnomalyDetected {
		score -= 0.1
	}

	// Clamp to valid range
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// GetAnomalyHistory returns anomaly history for a device
func (v *Validator) GetAnomalyHistory(deviceID string) []AnomalyRecord {
	return v.anomalyHistory[deviceID]
}

// ClearAnomalyHistory clears anomaly history for a device
func (v *Validator) ClearAnomalyHistory(deviceID string) {
	delete(v.anomalyHistory, deviceID)
}

// CleanOldAnomalies removes old anomaly records
func (v *Validator) CleanOldAnomalies() {
	now := time.Now()
	for deviceID := range v.anomalyHistory {
		history := v.anomalyHistory[deviceID]
		cleaned := make([]AnomalyRecord, 0)
		for _, record := range history {
			if now.Sub(record.FirstSeen) < v.AnomalyWindow*2 {
				cleaned = append(cleaned, record)
			}
		}
		if len(cleaned) == 0 {
			delete(v.anomalyHistory, deviceID)
		} else {
			v.anomalyHistory[deviceID] = cleaned
		}
	}
}

// abs returns absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// SortErrors sorts errors by severity
func (r *ValidationResult) SortErrors() {
	severityOrder := map[string]int{
		"critical": 0,
		"error":    1,
		"warning":  2,
	}

	sort.Slice(r.Errors, func(i, j int) bool {
		return severityOrder[r.Errors[i].Severity] < severityOrder[r.Errors[j].Severity]
	})
}
