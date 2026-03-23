package android

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tenkile/tenkile/internal/probes"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestNewAndroidAdapter(t *testing.T) {
	adapter := NewAndroidAdapter()
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.baseTrust != 0.85 {
		t.Errorf("expected base trust 0.85, got %f", adapter.baseTrust)
	}
}

func TestAndroidAdapter_ClientType(t *testing.T) {
	adapter := NewAndroidAdapter()
	if adapter.ClientType() != "android" {
		t.Errorf("expected 'android', got %q", adapter.ClientType())
	}
}

func TestAndroidAdapter_SourceID(t *testing.T) {
	adapter := NewAndroidAdapter()
	if adapter.SourceID() != "android-mediacodec" {
		t.Errorf("expected 'android-mediacodec', got %q", adapter.SourceID())
	}
}

func TestAndroidAdapter_TrustScore(t *testing.T) {
	adapter := NewAndroidAdapter()
	score := adapter.TrustScore()
	if score <= 0.0 || score > 1.0 {
		t.Errorf("trust score should be between 0 and 1, got %f", score)
	}
}

func TestAndroidAdapter_ExtractCapabilities_Headers(t *testing.T) {
	adapter := NewAndroidAdapter()

	tests := []struct {
		name       string
		setupReq   func(*http.Request)
		checkCaps  func(*testing.T, *probes.DeviceCapabilities)
	}{
		{
			name: "Device identity from headers",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Device-ID", "android-device-123")
				r.Header.Set("X-Android-Manufacturer", "Samsung")
				r.Header.Set("X-Android-Model", "Galaxy S21")
				r.Header.Set("X-Android-Version", "13")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.DeviceID != "android-device-123" {
					t.Errorf("expected device ID 'android-device-123', got %q", caps.DeviceID)
				}
				if caps.Manufacturer != "Samsung" {
					t.Errorf("expected manufacturer 'Samsung', got %q", caps.Manufacturer)
				}
				if caps.Model != "Galaxy S21" {
					t.Errorf("expected model 'Galaxy S21', got %q", caps.Model)
				}
			},
		},
		{
			name: "Video codecs from MIME types",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-Video-Codecs", "video/avc,video/hevc,video/av01")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.VideoCodecs) != 3 {
					t.Errorf("expected 3 video codecs, got %d", len(caps.VideoCodecs))
				}
				if !contains(caps.VideoCodecs, "h264") {
					t.Error("expected h264 codec")
				}
				if !contains(caps.VideoCodecs, "hevc") {
					t.Error("expected hevc codec")
				}
				if !contains(caps.VideoCodecs, "av1") {
					t.Error("expected av1 codec")
				}
			},
		},
		{
			name: "Audio codecs from MIME types",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-Audio-Codecs", "audio/mp4a-latm,audio/ac3,audio/eac3,audio/opus")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.AudioCodecs) != 4 {
					t.Errorf("expected 4 audio codecs, got %d", len(caps.AudioCodecs))
				}
				if !contains(caps.AudioCodecs, "aac") {
					t.Error("expected aac codec")
				}
				if !contains(caps.AudioCodecs, "ac3") {
					t.Error("expected ac3 codec")
				}
				if !contains(caps.AudioCodecs, "eac3") {
					t.Error("expected eac3 codec")
				}
				if !contains(caps.AudioCodecs, "opus") {
					t.Error("expected opus codec")
				}
			},
		},
		{
			name: "HDR capabilities",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-HDR", "hdr10,dolby-vision,hdr10plus,hlg")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if !caps.SupportsHDR {
					t.Error("expected HDR support")
				}
				if !caps.SupportsDolbyVision {
					t.Error("expected Dolby Vision support")
				}
				if !caps.Supports10Bit {
					t.Error("expected 10-bit support (HDR10+)")
				}
			},
		},
		{
			name: "DRM systems",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-DRM", "widevine,playready,clearkey")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.DRMSupport == nil {
					t.Fatal("expected DRM support")
				}
				if !caps.DRMSupport.Supported {
					t.Error("expected DRM to be supported")
				}
				systems := caps.DRMSupport.Systems
				if !contains(systems, "widevine") {
					t.Error("expected widevine")
				}
				if !contains(systems, "playready") {
					t.Error("expected playready")
				}
			},
		},
		{
			name: "Resolution from header",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-Resolution", "3840x2160")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.MaxWidth != 3840 {
					t.Errorf("expected width 3840, got %d", caps.MaxWidth)
				}
				if caps.MaxHeight != 2160 {
					t.Errorf("expected height 2160, got %d", caps.MaxHeight)
				}
			},
		},
		{
			name: "Max bitrate",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-Max-Bitrate", "100000000")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.MaxBitrate != 100000000 {
					t.Errorf("expected bitrate 100000000, got %d", caps.MaxBitrate)
				}
			},
		},
		{
			name: "Container formats",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Android-Containers", "video/mp4,video/x-matroska,video/webm")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.ContainerFormats) != 3 {
					t.Errorf("expected 3 container formats, got %d", len(caps.ContainerFormats))
				}
				if !contains(caps.ContainerFormats, "mp4") {
					t.Error("expected mp4")
				}
				if !contains(caps.ContainerFormats, "mkv") {
					t.Errorf("expected mkv, got: %v", caps.ContainerFormats)
				}
				if !contains(caps.ContainerFormats, "webm") {
					t.Error("expected webm")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tt.setupReq(req)

			caps, err := adapter.ExtractCapabilities(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if caps == nil {
				t.Fatal("expected non-nil capabilities")
			}

			tt.checkCaps(t, caps)
		})
	}
}

func TestMapAndroidCodec(t *testing.T) {
	tests := []struct {
		codec   string
		isVideo bool
		want    string
	}{
		// Video codecs
		{"video/avc", true, "h264"},
		{"video/hevc", true, "hevc"},
		{"video/av01", true, "av1"},
		{"avc", true, "h264"},
		{"hevc", true, "hevc"},
		{"av01", true, "av1"},
		{"vp9", true, "vp9"},
		{"vp8", true, "vp8"},

		// Audio codecs
		{"audio/mp4a-latm", false, "aac"},
		{"audio/ac3", false, "ac3"},
		{"audio/eac3", false, "eac3"},
		{"audio/opus", false, "opus"},
		{"audio/flac", false, "flac"},
		{"audio/mpeg", false, "mp3"},

		// Encoder names
		{"OMX.qcom.video.encoder.avc", true, "h264"},
		{"c2.android.hevc.encoder", true, "hevc"},
		{"OMX.google.aac.encoder", false, "aac"},
	}

	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			got := MapAndroidCodec(tt.codec, tt.isVideo)
			if got != tt.want {
				t.Errorf("MapAndroidCodec(%q, %v) = %q, want %q", tt.codec, tt.isVideo, got, tt.want)
			}
		})
	}
}

func TestMapAndroidContainer(t *testing.T) {
	tests := []struct {
		container string
		want      string
	}{
		{"video/mp4", "mp4"},
		{"video/x-matroska", "mkv"},
		{"mkv", "mkv"},
		{"video/webm", "webm"},
		{"webm", "webm"},
		{"video/mp2t", "ts"},
		{"video/avi", "avi"},
		{"video/quicktime", "mov"},
	}

	for _, tt := range tests {
		t.Run(tt.container, func(t *testing.T) {
			got := MapAndroidContainer(tt.container)
			if got != tt.want {
				t.Errorf("MapAndroidContainer(%q) = %q, want %q", tt.container, got, tt.want)
			}
		})
	}
}

func TestParseHDRType(t *testing.T) {
	tests := []struct {
		hdrType string
		want    HDRInfo
	}{
		{"hdr10", HDRInfo{SupportsHDR: true}},
		{"hdr10plus", HDRInfo{SupportsHDR: true, Supports10Bit: true}},
		{"dolby-vision", HDRInfo{SupportsHDR: true, SupportsDolbyVision: true}},
		{"dv", HDRInfo{SupportsHDR: true, SupportsDolbyVision: true}},
		{"hlg", HDRInfo{SupportsHDR: true, SupportsHLG: true}},
		{"unknown", HDRInfo{}},
	}

	for _, tt := range tests {
		t.Run(tt.hdrType, func(t *testing.T) {
			got := ParseHDRType(tt.hdrType)
			if got.SupportsHDR != tt.want.SupportsHDR {
				t.Errorf("SupportsHDR = %v, want %v", got.SupportsHDR, tt.want.SupportsHDR)
			}
			if got.SupportsDolbyVision != tt.want.SupportsDolbyVision {
				t.Errorf("SupportsDolbyVision = %v, want %v", got.SupportsDolbyVision, tt.want.SupportsDolbyVision)
			}
			if got.Supports10Bit != tt.want.Supports10Bit {
				t.Errorf("Supports10Bit = %v, want %v", got.Supports10Bit, tt.want.Supports10Bit)
			}
		})
	}
}

func TestParseMediaTypes(t *testing.T) {
	tests := []struct {
		input  string
		prefix string
		want   []string
	}{
		{"video/avc,video/hevc", "video/", []string{"h264", "hevc"}},
		{"audio/mp4a-latm,audio/ac3", "audio/", []string{"aac", "ac3"}},
		{"avc,hevc,vp9", "video/", []string{"h264", "hevc", "vp9"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseMediaTypes(tt.input, tt.prefix)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice []string
		val   string
		want  bool
	}{
		{[]string{"h264", "hevc", "av1"}, "hevc", true},
		{[]string{"h264", "hevc", "av1"}, "vp9", false},
		{[]string{}, "h264", false},
		{nil, "h264", false},
	}

	for _, tt := range tests {
		got := contains(tt.slice, tt.val)
		if got != tt.want {
			t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.val, got, tt.want)
		}
	}
}

func TestAndroidAdapter_DefaultCodecs(t *testing.T) {
	adapter := NewAndroidAdapter()

	// Request with no codec headers
	req := httptest.NewRequest("GET", "/", nil)
	caps, err := adapter.ExtractCapabilities(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have default codecs
	if len(caps.VideoCodecs) == 0 {
		t.Error("expected default video codecs")
	}
	if len(caps.AudioCodecs) == 0 {
		t.Error("expected default audio codecs")
	}
	if len(caps.ContainerFormats) == 0 {
		t.Error("expected default container formats")
	}
}

func TestAndroidAdapter_TrustScoreAdjustment(t *testing.T) {
	adapter := NewAndroidAdapter()

	// High capability report
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Android-Video-Codecs", "video/avc,video/hevc,video/av01")
	req.Header.Set("X-Android-HDR", "hdr10,dolby-vision")
	req.Header.Set("X-Android-DRM", "widevine")
	req.Header.Set("X-Android-Resolution", "3840x2160")

	caps, _ := adapter.ExtractCapabilities(req)

	// Trust should be higher due to rich capabilities
	if caps.TrustScore <= adapter.baseTrust {
		t.Errorf("expected trust score higher than base %f, got %f", adapter.baseTrust, caps.TrustScore)
	}
}

func TestAndroidAdapter_TrustScoreCapped(t *testing.T) {
	adapter := NewAndroidAdapter()

	// Maximum capability report
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Android-Video-Codecs", "video/avc,video/hevc,video/av01,vp9,vp8")
	req.Header.Set("X-Android-Audio-Codecs", "audio/mp4a-latm,audio/ac3,audio/eac3,audio/opus,audio/flac")
	req.Header.Set("X-Android-HDR", "hdr10,dolby-vision,hdr10plus")
	req.Header.Set("X-Android-DRM", "widevine,playready")
	req.Header.Set("X-Android-Resolution", "3840x2160")
	req.Header.Set("X-Android-Max-Bitrate", "200000000")

	caps, _ := adapter.ExtractCapabilities(req)

	// Trust should be capped at 0.98
	if caps.TrustScore > 0.98 {
		t.Errorf("expected trust score capped at 0.98, got %f", caps.TrustScore)
	}
}
