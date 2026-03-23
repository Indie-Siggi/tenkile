package clients

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tenkile/tenkile/internal/probes"
)

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

func TestNewDetector(t *testing.T) {
	d := NewDetector()
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.adapters == nil {
		t.Error("expected adapters map to be initialized")
	}
	if d.patterns == nil {
		t.Error("expected patterns slice to be initialized")
	}
}

func TestDetector_RegisterAdapter(t *testing.T) {
	d := NewDetector()
	adapter := NewWebClientAdapter()

	d.RegisterAdapter(adapter)

	if d.GetAdapter("web") == nil {
		t.Error("expected web adapter to be registered")
	}
}

func TestDetector_Detect_Android(t *testing.T) {
	d := NewDetector()
	d.RegisterAdapter(NewWebClientAdapter())

	tests := []struct {
		name      string
		header    string
		userAgent string
		want      string
	}{
		{
			name:   "X-Platform header",
			header: "android",
			want:   "android",
		},
		{
			name:      "User-Agent",
			userAgent: "Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36",
			want:      "android",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Platform", tt.header)
			}
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}

			platform := d.detectPlatform(req)
			if platform != tt.want {
				t.Errorf("expected %q, got %q", tt.want, platform)
			}
		})
	}
}

func TestDetector_Detect_iOS(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name      string
		header    string
		userAgent string
		want      string
	}{
		{
			name:   "X-Platform header",
			header: "ios",
			want:   "ios",
		},
		{
			name:      "iPhone User-Agent",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15",
			want:      "ios",
		},
		{
			name:      "iPad User-Agent",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X) AppleWebKit/605.1.15",
			want:      "ios",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Platform", tt.header)
			}
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}

			platform := d.detectPlatform(req)
			if platform != tt.want {
				t.Errorf("expected %q, got %q", tt.want, platform)
			}
		})
	}
}

func TestDetector_Detect_TVPlatforms(t *testing.T) {
	d := NewDetector()
	d.RegisterAdapter(NewWebClientAdapter())

	tests := []struct {
		name      string
		header    string
		userAgent string
		want      string
	}{
		{
			name:   "Apple TV via header",
			header: "appletvos",
			want:   "appletvos",
		},
		{
			name:      "Apple TV via UA",
			userAgent: "AppleTV6,2/15.1",
			want:      "appletvos",
		},
		{
			name:   "Tizen via header",
			header: "tizen",
			want:   "tizen",
		},
		{
			name:      "Tizen via UA",
			userAgent: "Mozilla/5.0 (Linux; Tizen 2.4) AppleWebKit/537.36",
			want:      "tizen",
		},
		{
			name:   "WebOS via header",
			header: "webos",
			want:   "webos",
		},
		{
			name:   "Roku via header",
			header: "roku",
			want:   "roku",
		},
		{
			name:      "Roku via UA",
			userAgent: "Roku/DVP-9.0 (9.0.0)",
			want:      "roku",
		},
		{
			name:   "FireTV via header",
			header: "firetv",
			want:   "firetv",
		},
		{
			name:      "FireTV via UA",
			userAgent: "Mozilla/5.0 (Linux; Android 9; AFTT Build/PPR1.180610.011; CloudTV) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3780.126 Safari/537.36",
			want:      "firetv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Platform", tt.header)
			}
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}

			platform := d.detectPlatform(req)
			if platform != tt.want {
				t.Errorf("expected %q, got %q", tt.want, platform)
			}
		})
	}
}

func TestDetector_Detect_WebBrowsers(t *testing.T) {
	d := NewDetector()
	d.RegisterAdapter(NewWebClientAdapter())

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "Chrome on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:      "chrome",
		},
		{
			name:      "Edge on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			want:      "edge",
		},
		{
			name:      "Firefox on Linux",
			userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			want:      "firefox",
		},
		{
			name:      "Safari on macOS",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
			want:      "safari",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("User-Agent", tt.userAgent)

			platform := d.detectPlatform(req)
			if platform != tt.want {
				t.Errorf("expected %q, got %q", tt.want, platform)
			}
		})
	}
}

func TestDetector_Detect_Fallback(t *testing.T) {
	d := NewDetector()
	d.RegisterAdapter(NewWebClientAdapter())

	req := httptest.NewRequest("GET", "/", nil)
	// No headers, no User-Agent

	adapter := d.Detect(req)
	if adapter == nil {
		t.Fatal("expected fallback to web adapter")
	}
	if adapter.ClientType() != "web" {
		t.Errorf("expected web adapter, got %q", adapter.ClientType())
	}
}

func TestWebClientAdapter_ClientType(t *testing.T) {
	adapter := NewWebClientAdapter()
	if adapter.ClientType() != "web" {
		t.Errorf("expected 'web', got %q", adapter.ClientType())
	}
}

func TestWebClientAdapter_SourceID(t *testing.T) {
	adapter := NewWebClientAdapter()
	if adapter.SourceID() != "tenkile:web-client" {
		t.Errorf("expected 'tenkile:web-client', got %q", adapter.SourceID())
	}
}

func TestWebClientAdapter_TrustScore(t *testing.T) {
	adapter := NewWebClientAdapter()
	score := adapter.TrustScore()
	if score <= 0.0 || score > 1.0 {
		t.Errorf("trust score should be between 0 and 1, got %f", score)
	}
}

func TestWebClientAdapter_ExtractCapabilities(t *testing.T) {
	adapter := NewWebClientAdapter()

	tests := []struct {
		name         string
		setupRequest func(*http.Request)
		checkCaps    func(*testing.T, *probes.DeviceCapabilities)
	}{
		{
			name: "Chrome browser",
			setupRequest: func(r *http.Request) {
				r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.Platform != "chrome" {
					t.Errorf("expected platform 'chrome', got %q", caps.Platform)
				}
				if caps.Identity.Platform.WebBrowser != "chrome" {
					t.Errorf("expected browser 'chrome', got %q", caps.Identity.Platform.WebBrowser)
				}
			},
		},
		{
			name: "Codec headers",
			setupRequest: func(r *http.Request) {
				r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0")
				r.Header.Set("X-Codec-Video", "h264,hevc,av1,vp9")
				r.Header.Set("X-Codec-Audio", "aac,mp3,flac,opus")
				r.Header.Set("X-Resolution", "3840x2160")
				r.Header.Set("X-HDR", "hdr10,dolby-vision")
				r.Header.Set("X-Max-Bitrate", "50000000")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if len(caps.VideoCodecs) != 4 {
					t.Errorf("expected 4 video codecs, got %d", len(caps.VideoCodecs))
				}
				if caps.MaxWidth != 3840 || caps.MaxHeight != 2160 {
					t.Errorf("expected 3840x2160, got %dx%d", caps.MaxWidth, caps.MaxHeight)
				}
				if !caps.SupportsHDR {
					t.Error("expected HDR support")
				}
				if !caps.SupportsDolbyVision {
					t.Error("expected Dolby Vision support")
				}
				if caps.MaxBitrate != 50000000 {
					t.Errorf("expected 50 Mbps bitrate, got %d", caps.MaxBitrate)
				}
			},
		},
		{
			name: "DRM headers",
			setupRequest: func(r *http.Request) {
				r.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0.0.0")
				r.Header.Set("X-DRM", "widevine,playready,fairplay")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.DRMSupport == nil {
					t.Fatal("expected DRM support")
				}
				if !caps.DRMSupport.Supported {
					t.Error("expected DRM to be supported")
				}
				systems := caps.DRMSupport.Systems
				if !containsStr(systems, "widevine") {
					t.Error("expected widevine in systems")
				}
				if !containsStr(systems, "playready") {
					t.Error("expected playready in systems")
				}
				if !containsStr(systems, "fairplay") {
					t.Error("expected fairplay in systems")
				}
			},
		},
		{
			name: "Device ID from header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0.0.0")
				r.Header.Set("X-Device-ID", "device-12345")
			},
			checkCaps: func(t *testing.T, caps *probes.DeviceCapabilities) {
				if caps.DeviceID != "device-12345" {
					t.Errorf("expected device ID 'device-12345', got %q", caps.DeviceID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tt.setupRequest(req)

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

func TestWebClientAdapter_DefaultCodecs(t *testing.T) {
	adapter := NewWebClientAdapter()

	tests := []struct {
		browser string
		check   func(*testing.T, []string)
	}{
		{
			browser: "chrome",
			check: func(t *testing.T, codecs []string) {
				if !contains(codecs, "h264") {
					t.Error("expected h264 for Chrome")
				}
			},
		},
		{
			browser: "firefox",
			check: func(t *testing.T, codecs []string) {
				// Firefox defaults
				if !contains(codecs, "h264") || !contains(codecs, "vp9") {
					t.Error("expected h264 and vp9 for Firefox")
				}
			},
		},
		{
			browser: "safari",
			check: func(t *testing.T, codecs []string) {
				// Safari defaults
				if !contains(codecs, "h264") || !contains(codecs, "hevc") {
					t.Error("expected h264 and hevc for Safari")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.browser, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			// Create a more realistic UA that triggers browser detection
			switch tt.browser {
			case "chrome":
				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			case "firefox":
				req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0")
			case "safari":
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15")
			}

			caps, _ := adapter.ExtractCapabilities(req)
			tt.check(t, caps.VideoCodecs)
		})
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Test that global registry is singleton
	r1 := GetRegistry()
	r2 := GetRegistry()
	if r1 != r2 {
		t.Error("expected singleton registry")
	}

	// Test package-level functions
	RegisterAdapter(NewWebClientAdapter())
	adapter := DetectClient(httptest.NewRequest("GET", "/", nil))
	if adapter == nil {
		t.Error("expected non-nil adapter")
	}

	platform := DetectPlatform(httptest.NewRequest("GET", "/", nil))
	if platform == "" {
		t.Error("expected non-empty platform")
	}
}

func TestRegistry_ListAdapters(t *testing.T) {
	r := NewRegistry()
	r.RegisterAdapter(NewWebClientAdapter())

	adapters := r.ListAdapters()
	if len(adapters) == 0 {
		t.Error("expected at least one adapter")
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

// containsStr is a helper for checking string slices
func containsStr(slice []string, val string) bool {
	return contains(slice, val)
}
