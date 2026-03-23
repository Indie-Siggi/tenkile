package stream

import (
	"testing"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

// --- Model Tests ---

func TestDefaultVariants(t *testing.T) {
	variants := DefaultVariants()

	if len(variants) != 5 {
		t.Errorf("expected 5 variants, got %d", len(variants))
	}

	// Verify expected variants in order
	expected := []struct {
		name           string
		width, height  int
		bitrate       int64
		audioBitrate  int64
	}{
		{"4k", 3840, 2160, 45_000_000, 320_000},
		{"1080p", 1920, 1080, 8_000_000, 192_000},
		{"720p", 1280, 720, 4_000_000, 128_000},
		{"480p", 854, 480, 2_500_000, 128_000},
		{"360p", 640, 360, 1_000_000, 96_000},
	}

	for i, exp := range expected {
		v := variants[i]
		if v.Name != exp.name {
			t.Errorf("variant[%d]: expected name %q, got %q", i, exp.name, v.Name)
		}
		if v.Width != exp.width {
			t.Errorf("variant[%d]: expected width %d, got %d", i, exp.width, v.Width)
		}
		if v.Height != exp.height {
			t.Errorf("variant[%d]: expected height %d, got %d", i, exp.height, v.Height)
		}
		if v.Bitrate != exp.bitrate {
			t.Errorf("variant[%d]: expected bitrate %d, got %d", i, exp.bitrate, v.Bitrate)
		}
		if v.AudioBitrate != exp.audioBitrate {
			t.Errorf("variant[%d]: expected audio_bitrate %d, got %d", i, exp.audioBitrate, v.AudioBitrate)
		}
	}
}

func TestStreamSession(t *testing.T) {
	session := &StreamSession{
		ID:          "session_123",
		MediaItemID: "media_456",
		UserID:      "user_789",
		DeviceID:    "device_abc",
		StreamType:  StreamTypeHLS,
		Variant:    "1080p",
	}

	if session.ID != "session_123" {
		t.Errorf("expected ID 'session_123', got %q", session.ID)
	}
	if session.MediaItemID != "media_456" {
		t.Errorf("expected MediaItemID 'media_456', got %q", session.MediaItemID)
	}
	if session.StreamType != StreamTypeHLS {
		t.Errorf("expected StreamType 'hls', got %q", session.StreamType)
	}
	if session.Variant != "1080p" {
		t.Errorf("expected Variant '1080p', got %q", session.Variant)
	}
}

func TestHLSOptions(t *testing.T) {
	opts := HLSOptions{
		SegmentDuration: 6,
		PlaylistSize:    0,
		StartNumber:    1,
		TempDir:        "/tmp/hls",
		IncludeAudio:   true,
	}

	if opts.SegmentDuration != 6 {
		t.Errorf("expected SegmentDuration 6, got %d", opts.SegmentDuration)
	}
	if opts.PlaylistSize != 0 {
		t.Errorf("expected PlaylistSize 0 (infinite), got %d", opts.PlaylistSize)
	}
	if !opts.IncludeAudio {
		t.Error("expected IncludeAudio true")
	}
}

func TestHLSOptionsDefaults(t *testing.T) {
	opts := HLSOptions{}

	// Zero values should be defaults that will be overridden
	if opts.SegmentDuration != 0 {
		t.Errorf("expected default SegmentDuration 0, got %d", opts.SegmentDuration)
	}
	if opts.IncludeAudio {
		t.Error("expected default IncludeAudio false")
	}
}

func TestStreamTypeConstants(t *testing.T) {
	tests := []struct {
		streamType StreamType
		expected   string
	}{
		{StreamTypeHLS, "hls"},
		{StreamTypeDASH, "dash"},
		{StreamTypeDirect, "direct"},
		{StreamTypeRemux, "remux"},
		{StreamTypeTranscode, "transcode"},
	}

	for _, tt := range tests {
		if string(tt.streamType) != tt.expected {
			t.Errorf("expected StreamType %q, got %q", tt.expected, string(tt.streamType))
		}
	}
}

func TestHLSManifest(t *testing.T) {
	manifest := &HLSManifest{
		MasterPlaylist: "/tmp/hls/master.m3u8",
		Variants: []VariantPlaylist{
			{Name: "1080p", Playlist: "/tmp/hls/1080p/playlist.m3u8", SegmentsDir: "/tmp/hls/1080p"},
		},
		TempDir: "/tmp/hls",
	}

	if manifest.MasterPlaylist != "/tmp/hls/master.m3u8" {
		t.Errorf("expected MasterPlaylist '/tmp/hls/master.m3u8', got %q", manifest.MasterPlaylist)
	}
	if len(manifest.Variants) != 1 {
		t.Errorf("expected 1 variant, got %d", len(manifest.Variants))
	}
	if manifest.Variants[0].Name != "1080p" {
		t.Errorf("expected variant name '1080p', got %q", manifest.Variants[0].Name)
	}
}

func TestVariantPlaylist(t *testing.T) {
	vp := VariantPlaylist{
		Name:        "720p",
		Playlist:    "/tmp/hls/720p/playlist.m3u8",
		SegmentsDir: "/tmp/hls/720p/segments",
	}

	if vp.Name != "720p" {
		t.Errorf("expected Name '720p', got %q", vp.Name)
	}
	if vp.Playlist != "/tmp/hls/720p/playlist.m3u8" {
		t.Errorf("expected Playlist '/tmp/hls/720p/playlist.m3u8', got %q", vp.Playlist)
	}
	if vp.SegmentsDir != "/tmp/hls/720p/segments" {
		t.Errorf("expected SegmentsDir '/tmp/hls/720p/segments', got %q", vp.SegmentsDir)
	}
}

func TestDASHManifest(t *testing.T) {
	manifest := &DASHManifest{
		ManifestPath: "/tmp/dash/manifest.mpd",
		TempDir:     "/tmp/dash",
	}

	if manifest.ManifestPath != "/tmp/dash/manifest.mpd" {
		t.Errorf("expected ManifestPath '/tmp/dash/manifest.mpd', got %q", manifest.ManifestPath)
	}
}

func TestDASHOptions(t *testing.T) {
	opts := DASHOptions{
		SegmentDuration: 4,
		TempDir:        "/tmp/dash",
	}

	if opts.SegmentDuration != 4 {
		t.Errorf("expected SegmentDuration 4, got %d", opts.SegmentDuration)
	}
	if opts.TempDir != "/tmp/dash" {
		t.Errorf("expected TempDir '/tmp/dash', got %q", opts.TempDir)
	}
}
