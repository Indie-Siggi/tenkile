// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"encoding/json"
	"testing"
)

// TestParseCodecProbe_ValidInput tests parsing valid probe JSON
func TestParseCodecProbe_ValidInput(t *testing.T) {
	input := `{
		"device_id": "test-device-001",
		"device_name": "Test Device",
		"platform": "ios",
		"os_version": "17.0",
		"app_version": "1.0.0",
		"model": "iPhone 15 Pro",
		"manufacturer": "Apple",
		"user_agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)",
		"video_codecs": [
			{"name": "h264", "profile": "high", "level": "5.1", "bit_depth": 8}
		],
		"audio_codecs": [
			{"name": "aac", "channels": 2, "sample_rates": [44100, 48000]}
		],
		"subtitle_formats": ["srt", "vtt"],
		"container_formats": ["mp4", "mkv"],
		"max_resolution": "1920x1080",
		"max_bitrate": 50000000,
		"supports_hdr": true,
		"supports_dolby_vision": false,
		"supports_dolby_atmos": false,
		"supports_dts": false,
		"probed_at": "2024-01-15T10:30:00Z",
		"probe_duration_ms": 150
	}`

	caps, err := ParseCodecProbe([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if caps == nil {
		t.Fatal("Expected non-nil capabilities")
	}

	if caps.DeviceID != "test-device-001" {
		t.Errorf("Expected device_id 'test-device-001', got '%s'", caps.DeviceID)
	}

	if caps.Platform != "ios" {
		t.Errorf("Expected platform 'ios', got '%s'", caps.Platform)
	}

	if len(caps.VideoCodecs) != 1 {
		t.Errorf("Expected 1 video codec, got %d", len(caps.VideoCodecs))
	}

	if caps.MaxWidth != 1920 || caps.MaxHeight != 1080 {
		t.Errorf("Expected resolution 1920x1080, got %dx%d", caps.MaxWidth, caps.MaxHeight)
	}
}

// TestParseCodecProbe_NilInput tests parsing nil/empty input
func TestParseCodecProbe_NilInput(t *testing.T) {
	_, err := ParseCodecProbe(nil)
	if err == nil {
		t.Error("Expected error for nil input")
	}

	_, err = ParseCodecProbe([]byte{})
	if err == nil {
		t.Error("Expected error for empty input")
	}

	_, err = ParseCodecProbe([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestParseCodecProbeResult_NilProbe tests parsing nil probe result
func TestParseCodecProbeResult_NilProbe(t *testing.T) {
	_, err := ParseCodecProbeResult(nil)
	if err == nil {
		t.Error("Expected error for nil probe result")
	}
}

// TestParseResolution_Valid tests parsing valid resolution strings
func TestParseResolution_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expectedWidth  int
		expectedHeight int
	}{
		{"1920x1080", 1920, 1080},
		{"3840x2160", 3840, 2160},
		{"1280x720", 1280, 720},
		{"  1920  x  1080  ", 1920, 1080},
	}

	for _, test := range tests {
		width, height, err := parseResolution(test.input)
		if err != nil {
			t.Errorf("parseResolution(%q) returned error: %v", test.input, err)
			continue
		}
		if width != test.expectedWidth {
			t.Errorf("parseResolution(%q): expected width %d, got %d", test.input, test.expectedWidth, width)
		}
		if height != test.expectedHeight {
			t.Errorf("parseResolution(%q): expected height %d, got %d", test.input, test.expectedHeight, height)
		}
	}
}

// TestParseResolution_Invalid tests parsing invalid resolution strings
func TestParseResolution_Invalid(t *testing.T) {
	tests := []string{
		"invalid",
		"1920",
		"x1080",
		"1920x1080x720",
		"abc-def",
	}

	for _, input := range tests {
		_, _, err := parseResolution(input)
		if err == nil {
			t.Errorf("Expected error for invalid resolution %q", input)
		}
	}
}

// TestDeviceCapabilities_ToJSON tests JSON serialization
func TestDeviceCapabilities_ToJSON(t *testing.T) {
	caps := &DeviceCapabilities{
		DeviceID:   "test-001",
		Platform:   "web",
		MaxWidth:   1920,
		MaxHeight:  1080,
		VideoCodecs: []string{"h264", "vp9"},
	}

	data, err := caps.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() returned error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result["device_id"] != "test-001" {
		t.Errorf("Expected device_id 'test-001', got %v", result["device_id"])
	}
}

// TestParseCodecProbe_MinimalInput tests parsing minimal valid input
func TestParseCodecProbe_MinimalInput(t *testing.T) {
	input := `{
		"device_id": "minimal-device",
		"platform": "android",
		"video_codecs": [],
		"audio_codecs": []
	}`

	caps, err := ParseCodecProbe([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error for minimal input, got: %v", err)
	}

	if caps.DeviceID != "minimal-device" {
		t.Errorf("Expected device_id 'minimal-device', got '%s'", caps.DeviceID)
	}
}

// TestParseCodecProbe_ComplexInput tests parsing complex input with all fields
func TestParseCodecProbe_ComplexInput(t *testing.T) {
	input := `{
		"device_id": "complex-device-001",
		"device_name": "Complex Test Device",
		"platform": "android",
		"os_version": "14.0",
		"app_version": "2.5.1",
		"model": "Pixel 8 Pro",
		"manufacturer": "Google",
		"user_agent": "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro)",
		"video_codecs": [
			{"name": "h264", "profile": "high", "level": "5.1", "bit_depth": 8, "max_width": 3840, "max_height": 2160},
			{"name": "hevc", "profile": "main10", "level": "5.1", "bit_depth": 10, "max_width": 3840, "max_height": 2160, "supports_hdr": true},
			{"name": "vp9", "profile": "profile2", "bit_depth": 10},
			{"name": "av1", "profile": "main", "bit_depth": 10}
		],
		"audio_codecs": [
			{"name": "aac", "channels": 2, "sample_rates": [44100, 48000], "bitrate": 256000},
			{"name": "flac", "channels": 8, "sample_rates": [48000], "supports_surround": true, "supports_truehd": false},
			{"name": "opus", "channels": 8, "supports_surround": true, "supports_atmos": false}
		],
		"subtitle_formats": ["srt", "vtt", "ass"],
		"container_formats": ["mp4", "mkv", "webm"],
		"drm_support": {
			"widevine": {"level": "l1", "supported": true},
			"playready": {"supported": false}
		},
		"max_resolution": "3840x2160",
		"max_bitrate": 80000000,
		"supports_hdr": true,
		"supports_dolby_vision": true,
		"supports_dolby_atmos": true,
		"supports_dts": false,
		"direct_play_support": {
			"video_containers": ["mp4", "mkv"],
			"audio_containers": ["mp4", "ogg"],
			"subtitle_formats": ["srt", "vtt"],
			"max_video_bitrate": 50000000,
			"max_audio_bitrate": 1000000
		},
		"transcoding_preferences": {
			"preferred_video_codec": "h264",
			"preferred_audio_codec": "aac",
			"preferred_container": "mp4",
			"max_transcode_bitrate": 25000000,
			"prefer_hardware_encoding": true
		},
		"probed_at": "2024-01-15T10:30:00Z",
		"probe_duration_ms": 250
	}`

	caps, err := ParseCodecProbe([]byte(input))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if caps.DeviceID != "complex-device-001" {
		t.Errorf("Expected device_id 'complex-device-001', got '%s'", caps.DeviceID)
	}

	if caps.SupportsHDR != true {
		t.Error("Expected SupportsHDR to be true")
	}

	if caps.SupportsDolbyVision != true {
		t.Error("Expected SupportsDolbyVision to be true")
	}

	if caps.SupportsDolbyAtmos != true {
		t.Error("Expected SupportsDolbyAtmos to be true")
	}

	if caps.MaxWidth != 3840 || caps.MaxHeight != 2160 {
		t.Errorf("Expected 4K resolution, got %dx%d", caps.MaxWidth, caps.MaxHeight)
	}

	if caps.DRMSupport == nil {
		t.Error("Expected non-nil DRMSupport")
	}

	if caps.DirectPlaySupport == nil {
		t.Error("Expected non-nil DirectPlaySupport")
	}

	if caps.TranscodingPreferences == nil {
		t.Error("Expected non-nil TranscodingPreferences")
	}
}
