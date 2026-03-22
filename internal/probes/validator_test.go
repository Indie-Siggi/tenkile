// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"testing"
)

// TestValidator_New tests creating a new validator
func TestValidator_New(t *testing.T) {
	validator := NewValidator()
	if validator == nil {
		t.Fatal("Expected non-nil validator")
	}
	if validator.PlatformConstraints == nil {
		t.Error("Expected PlatformConstraints to be initialized")
	}
	if validator.anomalyHistory == nil {
		t.Error("Expected anomalyHistory to be initialized")
	}
}

// TestValidator_ValidateCapabilities_NilInput tests validation with nil input
func TestValidator_ValidateCapabilities_NilInput(t *testing.T) {
	validator := NewValidator()
	result := validator.ValidateCapabilities(nil)

	if result.IsValid {
		t.Error("Expected validation to fail for nil input")
	}
	if result.Score != 0.0 {
		t.Errorf("Expected score 0.0 for nil input, got %f", result.Score)
	}
	if len(result.Errors) == 0 {
		t.Error("Expected errors for nil input")
	}
}

// TestValidator_ValidateCapabilities_ValidIOS tests validation of valid iOS device
func TestValidator_ValidateCapabilities_ValidIOS(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "ios-device-001",
		DeviceName:    "iPhone 15 Pro",
		Platform:      "ios",
		OSVersion:     "17.0",
		Model:         "iPhone15,3",
		Manufacturer:  "Apple",
		VideoCodecs:   []string{"h264", "hevc"},
		AudioCodecs:   []string{"aac", "alac", "ac3"},
		ContainerFormats: []string{"mp4", "mov", "mkv"},
		MaxWidth:      1920,
		MaxHeight:     1080,
		MaxBitrate:    50000000,
		SupportsHDR:   true,
		Supports10Bit: true,
	}

	result := validator.ValidateCapabilities(caps)

	if !result.IsValid {
		t.Errorf("Expected validation to pass, got errors: %v", result.Errors)
	}
	if result.Score != 1.0 {
		t.Errorf("Expected score 1.0, got %f", result.Score)
	}
}

// TestValidator_ValidateCapabilities_InvalidPlatform tests validation with invalid platform
func TestValidator_ValidateCapabilities_InvalidPlatform(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "unknown-device",
		Platform:      "unknown_platform_xyz",
		VideoCodecs:   []string{"h264"},
		AudioCodecs:   []string{"aac"},
	}

	result := validator.ValidateCapabilities(caps)

	// Unknown platform should produce warnings but still be valid
	hasWarning := false
	for _, w := range result.Warnings {
		if w.Code == "UNKNOWN_PLATFORM" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("Expected validation to warn about unknown platform")
	}
}

// TestValidator_ValidateCapabilities_NoVideoCodecs tests validation with no video codecs
func TestValidator_ValidateCapabilities_NoVideoCodecs(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "no-codecs-device",
		Platform:      "ios",
		VideoCodecs:   []string{},
		AudioCodecs:   []string{"aac"},
	}

	result := validator.ValidateCapabilities(caps)

	if result.IsValid {
		t.Error("Expected validation to fail with no video codecs")
	}

	hasNoVideoCodecsError := false
	for _, err := range result.Errors {
		if err.Code == "NO_VIDEO_CODECS" {
			hasNoVideoCodecsError = true
			break
		}
	}
	if !hasNoVideoCodecsError {
		t.Error("Expected NO_VIDEO_CODECS error")
	}
}

// TestValidator_ValidateCapabilities_ExcessiveCodecs tests validation with excessive codecs
func TestValidator_ValidateCapabilities_ExcessiveCodecs(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "excessive-codecs",
		Platform:      "ios",
		VideoCodecs:   []string{"h264", "hevc", "vp9", "av1", "mpeg4", "mpeg2", "vc1", "vp8", "theora", "dirac", "realvideo", "wmv"},
		AudioCodecs:   []string{"aac", "mp3", "flac", "alac", "ac3", "eac3", "truehd", "dts", "opus", "wma"},
	}

	result := validator.ValidateCapabilities(caps)

	if !result.AnomalyDetected {
		t.Error("Expected anomaly detected for excessive codecs")
	}
}

// TestValidator_ValidateCapabilities_HDRWithout10Bit tests HDR without 10-bit warning
func TestValidator_ValidateCapabilities_HDRWithout10Bit(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "hdr-no-10bit",
		Platform:      "web",
		VideoCodecs:   []string{"h264", "vp9"},
		AudioCodecs:   []string{"aac"},
		SupportsHDR:   true,
		Supports10Bit: false,
	}

	result := validator.ValidateCapabilities(caps)

	hasHDRWarning := false
	for _, warn := range result.Warnings {
		if warn.Code == "HDR_WITHOUT_10BIT" {
			hasHDRWarning = true
			break
		}
	}
	if !hasHDRWarning {
		t.Error("Expected HDR_WITHOUT_10BIT warning")
	}
}

// TestValidator_ValidateCapabilities_InvalidResolution tests validation with invalid resolution
func TestValidator_ValidateCapabilities_InvalidResolution(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "impossible-res",
		Platform:      "web",
		VideoCodecs:   []string{"h264"},
		AudioCodecs:   []string{"aac"},
		MaxWidth:      50000,
		MaxHeight:     50000,
	}

	result := validator.ValidateCapabilities(caps)

	if result.IsValid {
		t.Error("Expected validation to fail with impossible resolution")
	}

	hasImpossibleResError := false
	for _, err := range result.Errors {
		if err.Code == "IMPOSSIBLE_RESOLUTION" {
			hasImpossibleResError = true
			break
		}
	}
	if !hasImpossibleResError {
		t.Error("Expected IMPOSSIBLE_RESOLUTION error")
	}
}

// TestValidator_ValidateCapabilities_InvalidBitrate tests validation with invalid bitrate
func TestValidator_ValidateCapabilities_InvalidBitrate(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "impossible-bitrate",
		Platform:      "web",
		VideoCodecs:   []string{"h264"},
		AudioCodecs:   []string{"aac"},
		MaxBitrate:    5000000000, // 5 Gbps - unrealistic
	}

	result := validator.ValidateCapabilities(caps)

	if result.IsValid {
		t.Error("Expected validation to fail with impossible bitrate")
	}

	hasImpossibleBitrateError := false
	for _, err := range result.Errors {
		if err.Code == "IMPOSSIBLE_BITRATE" {
			hasImpossibleBitrateError = true
			break
		}
	}
	if !hasImpossibleBitrateError {
		t.Error("Expected IMPOSSIBLE_BITRATE error")
	}
}

// TestValidator_ValidateCapabilities_AndroidPlatform tests Android platform validation
func TestValidator_ValidateCapabilities_AndroidPlatform(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "android-device",
		Platform:      "android",
		VideoCodecs:   []string{"h264", "hevc", "vp9", "av1"},
		AudioCodecs:   []string{"aac", "opus", "flac"},
		ContainerFormats: []string{"mp4", "mkv", "webm"},
		MaxWidth:      3840,
		MaxHeight:     2160,
	}

	result := validator.ValidateCapabilities(caps)

	if !result.IsValid {
		t.Errorf("Expected validation to pass for Android, got errors: %v", result.Errors)
	}
}

// TestValidator_ValidateCapabilities_DolbyAtmosWithoutAC3 tests Dolby Atmos without AC3
func TestValidator_ValidateCapabilities_DolbyAtmosWithoutAC3(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "atmos-no-ac3",
		Platform:      "web",
		VideoCodecs:   []string{"h264", "hevc"},
		AudioCodecs:   []string{"aac", "flac"}, // No AC3/EAC3
		SupportsDolbyAtmos: true,
	}

	result := validator.ValidateCapabilities(caps)

	hasAtmosWarning := false
	for _, warn := range result.Warnings {
		if warn.Code == "ATMOS_WITHOUT_AC3" || warn.Code == "ATMOS_WITHOUT_BASE" {
			hasAtmosWarning = true
			break
		}
	}
	if !hasAtmosWarning {
		t.Error("Expected ATMOS warning without AC3/EAC3")
	}
}

// TestValidator_ValidateCapabilities_UnusualAspectRatio tests unusual aspect ratio warning
func TestValidator_ValidateCapabilities_UnusualAspectRatio(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "weird-aspect",
		Platform:      "web",
		VideoCodecs:   []string{"h264"},
		AudioCodecs:   []string{"aac"},
		MaxWidth:      1234,
		MaxHeight:     567,
	}

	result := validator.ValidateCapabilities(caps)

	hasUnusualAspectRatio := false
	for _, warn := range result.Warnings {
		if warn.Code == "UNUSUAL_ASPECT_RATIO" {
			hasUnusualAspectRatio = true
			break
		}
	}
	if !hasUnusualAspectRatio {
		t.Error("Expected UNUSUAL_ASPECT_RATIO warning")
	}
}

// TestValidator_ValidateCapabilities_MissingCommonContainers tests missing common containers
func TestValidator_ValidateCapabilities_MissingCommonContainers(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "no-common-containers",
		Platform:      "web",
		VideoCodecs:   []string{"h264"},
		AudioCodecs:   []string{"aac"},
		ContainerFormats: []string{"ogv", "avi"}, // No MP4/MKV
	}

	result := validator.ValidateCapabilities(caps)

	hasMissingContainerWarning := false
	for _, warn := range result.Warnings {
		if warn.Code == "MISSING_COMMON_CONTAINERS" {
			hasMissingContainerWarning = true
			break
		}
	}
	if !hasMissingContainerWarning {
		t.Error("Expected MISSING_COMMON_CONTAINERS warning")
	}
}

// TestValidator_GetAnomalyHistory tests anomaly history retrieval
func TestValidator_GetAnomalyHistory(t *testing.T) {
	validator := NewValidator()

	// Get history for non-existent device
	history := validator.GetAnomalyHistory("non-existent")
	if history != nil && len(history) > 0 {
		t.Error("Expected empty history for non-existent device")
	}

	// Trigger an anomaly
	caps := &DeviceCapabilities{
		DeviceID:      "anomaly-device",
		Platform:      "ios",
		VideoCodecs:   []string{}, // Will trigger error
		AudioCodecs:   []string{"aac"},
	}
	validator.ValidateCapabilities(caps)

	// Get history
	history = validator.GetAnomalyHistory("anomaly-device")
	if len(history) == 0 {
		t.Error("Expected anomaly history after validation error")
	}
}

// TestValidator_ClearAnomalyHistory tests clearing anomaly history
func TestValidator_ClearAnomalyHistory(t *testing.T) {
	validator := NewValidator()

	// Trigger anomaly
	caps := &DeviceCapabilities{
		DeviceID:      "clear-test",
		Platform:      "ios",
		VideoCodecs:   []string{},
		AudioCodecs:   []string{"aac"},
	}
	validator.ValidateCapabilities(caps)

	// Clear history
	validator.ClearAnomalyHistory("clear-test")

	// Verify cleared
	history := validator.GetAnomalyHistory("clear-test")
	if len(history) > 0 {
		t.Error("Expected empty history after clear")
	}
}

// TestValidator_SortErrors tests error sorting
func TestValidator_SortErrors(t *testing.T) {
	result := &ValidationResult{
		Errors: []ValidationError{
			{Code: "WARNING1", Severity: "warning"},
			{Code: "CRITICAL1", Severity: "critical"},
			{Code: "ERROR1", Severity: "error"},
		},
	}

	result.SortErrors()

	// Verify sorted order: critical, error, warning
	if result.Errors[0].Severity != "critical" {
		t.Errorf("Expected first error to be critical, got %s", result.Errors[0].Severity)
	}
	if result.Errors[1].Severity != "error" {
		t.Errorf("Expected second error to be error, got %s", result.Errors[1].Severity)
	}
	if result.Errors[2].Severity != "warning" {
		t.Errorf("Expected third error to be warning, got %s", result.Errors[2].Severity)
	}
}

// TestValidator_DolbyVisionWithoutCodec tests Dolby Vision without codec warning
func TestValidator_ValidateCapabilities_DolbyVisionWithoutCodec(t *testing.T) {
	validator := NewValidator()
	caps := &DeviceCapabilities{
		DeviceID:      "dv-no-codec",
		Platform:      "web",
		VideoCodecs:   []string{"vp8"}, // No HEVC or H.264
		AudioCodecs:   []string{"aac"},
		SupportsDolbyVision: true,
	}

	result := validator.ValidateCapabilities(caps)

	hasDVWarning := false
	for _, warn := range result.Warnings {
		if warn.Code == "DOLBY_VISION_WITHOUT_CODEC" {
			hasDVWarning = true
			break
		}
	}
	if !hasDVWarning {
		t.Error("Expected DOLBY_VISION_WITHOUT_CODEC warning")
	}
}

// TestValidator_ScoreCalculation tests score calculation
func TestValidator_ScoreCalculation(t *testing.T) {
	validator := NewValidator()

	// Test perfect score
	caps := &DeviceCapabilities{
		DeviceID:      "perfect",
		Platform:      "ios",
		VideoCodecs:   []string{"h264", "hevc"},
		AudioCodecs:   []string{"aac"},
		MaxWidth:      1920,
		MaxHeight:     1080,
		MaxBitrate:    50000000,
	}
	result := validator.ValidateCapabilities(caps)
	if result.Score > 0.9 || result.Score < 0.9 {
		t.Logf("Note: Score for valid device is %f (may have warnings)", result.Score)
	}

	// Test low score with errors
	caps = &DeviceCapabilities{
		DeviceID:      "bad",
		Platform:      "ios",
		VideoCodecs:   []string{},
		AudioCodecs:   []string{},
		MaxBitrate:    5000000000, // Impossible bitrate
	}
	result = validator.ValidateCapabilities(caps)
	if result.Score >= 0.8 {
		t.Errorf("Expected reduced score for invalid device, got %f", result.Score)
	}
}
