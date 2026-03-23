package apple

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tenkile/tenkile/internal/probes"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestNewIOSAdapter(t *testing.T) {
	adapter := NewIOSAdapter()
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.platform != "ios" {
		t.Errorf("expected platform 'ios', got %q", adapter.platform)
	}
}

func TestIOSAdapter_ClientType(t *testing.T) {
	adapter := NewIOSAdapter()
	if adapter.ClientType() != "ios" {
		t.Errorf("expected 'ios', got %q", adapter.ClientType())
	}
}

func TestIOSAdapter_SourceID(t *testing.T) {
	adapter := NewIOSAdapter()
	if adapter.SourceID() != "ios-avfoundation" {
		t.Errorf("expected 'ios-avfoundation', got %q", adapter.SourceID())
	}
}

func TestIOSAdapter_TrustScore(t *testing.T) {
	adapter := NewIOSAdapter()
	score := adapter.TrustScore()
	if score <= 0.0 || score > 1.0 {
		t.Errorf("trust score should be between 0 and 1, got %f", score)
	}
}

func TestNewTVOSAdapter(t *testing.T) {
	adapter := NewTVOSAdapter()
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.platform != "appletvos" {
		t.Errorf("expected platform 'appletvos', got %q", adapter.platform)
	}
}

func TestTVOSAdapter_ClientType(t *testing.T) {
	adapter := NewTVOSAdapter()
	if adapter.ClientType() != "appletvos" {
		t.Errorf("expected 'appletvos', got %q", adapter.ClientType())
	}
}

func TestTVOSAdapter_SourceID(t *testing.T) {
	adapter := NewTVOSAdapter()
	if adapter.SourceID() != "tvos-avfoundation" {
		t.Errorf("expected 'tvos-avfoundation', got %q", adapter.SourceID())
	}
}

func TestTVOSAdapter_TrustScore(t *testing.T) {
	adapter := NewTVOSAdapter()
	score := adapter.TrustScore()
	if score <= 0.0 || score > 1.0 {
		t.Errorf("trust score should be between 0 and 1, got %f", score)
	}
}

func TestIOSAdapter_ExtractCapabilities(t *testing.T) {
	adapter := NewIOSAdapter()

	tests := []struct {
		name      string
		setupReq  func(*http.Request)
		checkCaps func(*testing.T, *probes.DeviceCapabilities)
	}{
		{
			name: "Device identity from headers",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Device-ID", "ios-device-123")
				r.Header.Set("X-Apple-Model", "iPhone14,5")
				r.Header.Set("X-Apple-Version", "17.0")
				r.Header.Set("X-App-Version", "1.0.0")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.DeviceID != "ios-device-123" {
					t.Errorf("expected device ID 'ios-device-123', got %q", caps.DeviceID)
				}
				if caps.Manufacturer != "Apple" {
					t.Errorf("expected manufacturer 'Apple', got %q", caps.Manufacturer)
				}
				if caps.Model != "iPhone14,5" {
					t.Errorf("expected model 'iPhone14,5', got %q", caps.Model)
				}
				if caps.OSVersion != "17.0" {
					t.Errorf("expected OS version '17.0', got %q", caps.OSVersion)
				}
			},
		},
		{
			name: "Video codecs from headers",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Apple-Video-Codecs", "hvc1,avc1,av01")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.VideoCodecs) == 0 {
					t.Fatal("expected video codecs")
				}
				if !contains(caps.VideoCodecs, "hevc") {
					t.Error("expected hevc codec")
				}
				if !contains(caps.VideoCodecs, "h264") {
					t.Error("expected h264 codec")
				}
				if !contains(caps.VideoCodecs, "av1") {
					t.Error("expected av1 codec")
				}
			},
		},
		{
			name: "Audio codecs from headers",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Apple-Audio-Codecs", "aac,alac,ac3,eac3")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.AudioCodecs) == 0 {
					t.Fatal("expected audio codecs")
				}
				if !contains(caps.AudioCodecs, "aac") {
					t.Error("expected aac codec")
				}
				if !contains(caps.AudioCodecs, "alac") {
					t.Error("expected alac codec")
				}
			},
		},
		{
			name: "HDR capabilities",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Apple-HDR", "hdr10,dolby-vision")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if !caps.SupportsHDR {
					t.Error("expected HDR support")
				}
				if !caps.SupportsDolbyVision {
					t.Error("expected Dolby Vision support")
				}
			},
		},
		{
			name: "DRM systems",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Apple-DRM", "fairplay")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.DRMSupport == nil {
					t.Fatal("expected DRM support")
				}
				if !caps.DRMSupport.Supported {
					t.Error("expected DRM to be supported")
				}
				if !contains(caps.DRMSupport.Systems, "fairplay") {
					t.Error("expected fairplay")
				}
			},
		},
		{
			name: "Resolution from header",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Apple-Resolution", "3840x2160")
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
			name: "Container formats",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Apple-Containers", "mov,mp4,m4v")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.ContainerFormats) == 0 {
					t.Fatal("expected container formats")
				}
				if !contains(caps.ContainerFormats, "mov") {
					t.Error("expected mov")
				}
				if !contains(caps.ContainerFormats, "mp4") {
					t.Error("expected mp4")
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

func TestTVOSAdapter_ExtractCapabilities(t *testing.T) {
	adapter := NewTVOSAdapter()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Apple-Model", "AppleTV6,2")
	req.Header.Set("X-Apple-HDR", "dolby-vision")

	caps, err := adapter.ExtractCapabilities(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if caps.Platform != "appletvos" {
		t.Errorf("expected platform 'appletvos', got %q", caps.Platform)
	}

	if caps.Identity.Platform.Type != "smart_tv" {
		t.Errorf("expected platform type 'smart_tv', got %q", caps.Identity.Platform.Type)
	}
}

func TestMapAppleCodec(t *testing.T) {
	tests := []struct {
		codec   string
		isVideo bool
		want    string
	}{
		// Video codecs
		{"avc1", true, "h264"},
		{"hvc1", true, "hevc"},
		{"av01", true, "av1"},
		{"prores", true, "prores"},
		{"apch", true, "prores"},

		// Audio codecs
		{"aac", false, "aac"},
		{"alac", false, "alac"},
		{"ac3", false, "ac3"},
		{"eac3", false, "eac3"},
		{"flac", false, "flac"},
		{"opus", false, "opus"},

		// Codec with profile
		{"avc1.64001f", true, "h264"},
		{"hvc1.1.6.L120.90", true, "hevc"},
	}

	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			got := MapAppleCodec(tt.codec, tt.isVideo)
			if got != tt.want {
				t.Errorf("MapAppleCodec(%q, %v) = %q, want %q", tt.codec, tt.isVideo, got, tt.want)
			}
		})
	}
}

func TestMapAppleContainer(t *testing.T) {
	tests := []struct {
		container string
		want      string
	}{
		{"mov", "mov"},
		{"mp4", "mp4"},
		{"m4v", "m4v"},
		{"mkv", "mkv"},
		{"webm", "webm"},
	}

	for _, tt := range tests {
		t.Run(tt.container, func(t *testing.T) {
			got := MapAppleContainer(tt.container)
			if got != tt.want {
				t.Errorf("MapAppleContainer(%q) = %q, want %q", tt.container, got, tt.want)
			}
		})
	}
}

func TestParseAppleHDRType(t *testing.T) {
	tests := []struct {
		hdrType       string
		wantHDR       bool
		wantDV        bool
		want10Bit     bool
	}{
		{"hdr10", true, false, false},
		{"dolby-vision", true, true, false},
		{"hdr10plus", true, false, true},
		{"hlg", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.hdrType, func(t *testing.T) {
			hdr, dv, bit := ParseAppleHDRType(tt.hdrType)
			if hdr != tt.wantHDR {
				t.Errorf("HDR = %v, want %v", hdr, tt.wantHDR)
			}
			if dv != tt.wantDV {
				t.Errorf("DV = %v, want %v", dv, tt.wantDV)
			}
			if bit != tt.want10Bit {
				t.Errorf("10Bit = %v, want %v", bit, tt.want10Bit)
			}
		})
	}
}

func TestParseCodecList(t *testing.T) {
	tests := []struct {
		input   string
		isVideo bool
		want    []string
	}{
		{"hvc1,avc1,av01", true, []string{"hevc", "h264", "av1"}},
		{"aac,alac,ac3", false, []string{"aac", "alac", "ac3"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseCodecList(tt.input, tt.isVideo)
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

func TestParseContainerList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"mov,mp4,m4v", []string{"mov", "mp4", "m4v"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseContainerList(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
		})
	}
}

func TestAppleAdapter_DefaultCodecs(t *testing.T) {
	// Test iOS defaults
	iosAdapter := NewIOSAdapter()
	req := httptest.NewRequest("GET", "/", nil)
	caps, _ := iosAdapter.ExtractCapabilities(req)

	if len(caps.VideoCodecs) == 0 {
		t.Error("iOS: expected default video codecs")
	}
	if len(caps.AudioCodecs) == 0 {
		t.Error("iOS: expected default audio codecs")
	}
	if !contains(caps.VideoCodecs, "hevc") {
		t.Error("iOS: expected HEVC in defaults")
	}

	// Test tvOS defaults
	tvosAdapter := NewTVOSAdapter()
	caps, _ = tvosAdapter.ExtractCapabilities(req)

	if len(caps.VideoCodecs) == 0 {
		t.Error("tvOS: expected default video codecs")
	}
	if !contains(caps.AudioCodecs, "eac3") {
		t.Error("tvOS: expected EAC3 in defaults")
	}
}

func TestAppleAdapter_TrustScoreAdjustment(t *testing.T) {
	adapter := NewIOSAdapter()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Apple-Video-Codecs", "hvc1,avc1")
	req.Header.Set("X-Apple-HDR", "dolby-vision")
	req.Header.Set("X-Apple-DRM", "fairplay")

	caps, _ := adapter.ExtractCapabilities(req)

	// Trust should be higher due to rich capabilities
	if caps.TrustScore <= adapter.baseTrust {
		t.Errorf("expected trust score higher than base %f, got %f", adapter.baseTrust, caps.TrustScore)
	}
}
