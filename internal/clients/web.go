// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package clients

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/tenkile/tenkile/internal/probes"
)

// WebClientAdapter extracts device capabilities from web browser requests
type WebClientAdapter struct {
	// Base trust score for web adapters
	baseTrust float64
}

// NewWebClientAdapter creates a new web client adapter
func NewWebClientAdapter() *WebClientAdapter {
	return &WebClientAdapter{
		baseTrust: 0.60, // Base trust score for browser-reported capabilities
	}
}

// ClientType returns "web"
func (w *WebClientAdapter) ClientType() string {
	return "web"
}

// SourceID returns "tenkile:web-client"
func (w *WebClientAdapter) SourceID() string {
	return "tenkile:web-client"
}

// TrustScore returns the trust score adjusted for browser consensus
func (w *WebClientAdapter) TrustScore() float64 {
	return w.baseTrust
}

// ExtractCapabilities extracts capabilities from web browser request
func (w *WebClientAdapter) ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error) {
	caps := &probes.DeviceCapabilities{
		Platform: "web_browser",
	}

	// Extract device ID from headers or cookies
	caps.DeviceID = r.Header.Get("X-Device-ID")
	if caps.DeviceID == "" {
		caps.DeviceID = r.Header.Get("X-DeviceId")
	}

	// Extract User-Agent for browser detection
	ua := r.UserAgent()
	caps.UserAgent = ua

	// Detect browser and set browser-specific capabilities
	w.detectBrowser(r, caps)

	// Extract CodecProbe results from headers
	w.extractCodecHeaders(r, caps)

	// Try to extract full probe report from request body
	if r.ContentLength > 0 && r.ContentLength < 1<<20 { // Max 1MB
		if err := w.extractProbeBody(r, caps); err != nil {
			// Body extraction failed, but we have header data
		}
	}

	// Apply browser-specific defaults if not already set
	w.applyBrowserDefaults(caps)

	// Set trust score based on consensus level
	w.calculateTrustScore(caps)

	return caps, nil
}

// detectBrowser identifies the browser from User-Agent and headers
func (w *WebClientAdapter) detectBrowser(r *http.Request, caps *probes.DeviceCapabilities) {
	ua := r.UserAgent()

	// Chrome/Chromium-based browsers
	chromeRe := regexp.MustCompile(`(?i)Chrome/(\d+)\.`)
	if match := chromeRe.FindStringSubmatch(ua); len(match) > 1 {
		if strings.Contains(ua, "Edg/") || strings.Contains(ua, "Edge/") {
			caps.Platform = "edge"
			caps.Identity.Platform.WebBrowser = "edge"
		} else {
			caps.Platform = "chrome"
			caps.Identity.Platform.WebBrowser = "chrome"
		}
		if v, _ := strconv.Atoi(match[1]); v > 0 {
			caps.OSVersion = match[1]
		}
		return
	}

	// Firefox
	firefoxRe := regexp.MustCompile(`(?i)Firefox/(\d+)\.`)
	if match := firefoxRe.FindStringSubmatch(ua); len(match) > 1 {
		caps.Platform = "firefox"
		caps.Identity.Platform.WebBrowser = "firefox"
		if v, _ := strconv.Atoi(match[1]); v > 0 {
			caps.OSVersion = match[1]
		}
		return
	}

	// Safari (not Chrome)
	if strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/") {
		caps.Platform = "safari"
		caps.Identity.Platform.WebBrowser = "safari"
		// Safari version from version number
		safariRe := regexp.MustCompile(`(?i)Version/(\d+)\.`)
		if match := safariRe.FindStringSubmatch(ua); len(match) > 1 {
			caps.OSVersion = match[1]
		}
		return
	}

	// Samsung Browser
	if strings.Contains(ua, "SamsungBrowser") {
		caps.Platform = "samsung_browser"
		caps.Identity.Platform.WebBrowser = "samsung_browser"
		return
	}

	// LG TV Browser
	if strings.Contains(ua, "LG Browser") || strings.Contains(ua, "NetCast") {
		caps.Platform = "webos"
		caps.Identity.Platform.WebBrowser = "lg_browser"
		return
	}

	// Roku WebKit
	if strings.Contains(ua, "Roku") {
		caps.Platform = "roku"
		caps.Identity.Platform.WebBrowser = "roku_browser"
		return
	}

	// Default to generic web
	caps.Platform = "web_browser"
	caps.Identity.Platform.WebBrowser = "unknown"
}

// extractCodecHeaders extracts codec support from request headers
func (w *WebClientAdapter) extractCodecHeaders(r *http.Request, caps *probes.DeviceCapabilities) {
	// Video codecs header: X-Codec-Video: h264,hevc,av1,vp9
	if videoCodecs := r.Header.Get("X-Codec-Video"); videoCodecs != "" {
		codecs := strings.Split(strings.ToLower(videoCodecs), ",")
		for i := range codecs {
			codecs[i] = strings.TrimSpace(codecs[i])
		}
		caps.VideoCodecs = codecs
	}

	// Audio codecs header: X-Codec-Audio: aac,mp3,flac,opus
	if audioCodecs := r.Header.Get("X-Codec-Audio"); audioCodecs != "" {
		codecs := strings.Split(strings.ToLower(audioCodecs), ",")
		for i := range codecs {
			codecs[i] = strings.TrimSpace(codecs[i])
		}
		caps.AudioCodecs = codecs
	}

	// Container formats: X-Container: mp4,mkv,webm
	if containers := r.Header.Get("X-Container"); containers != "" {
		formats := strings.Split(strings.ToLower(containers), ",")
		for i := range formats {
			formats[i] = strings.TrimSpace(formats[i])
		}
		caps.ContainerFormats = formats
	}

	// HDR support: X-HDR: true / X-HDR: hdr10,dolby-vision
	if hdr := r.Header.Get("X-HDR"); hdr != "" {
		caps.SupportsHDR = strings.ToLower(hdr) == "true" || strings.Contains(strings.ToLower(hdr), "hdr")
		caps.SupportsDolbyVision = strings.Contains(strings.ToLower(hdr), "dolby") || strings.Contains(strings.ToLower(hdr), "dv")
	}

	// Max resolution: X-Resolution: 1920x1080
	if res := r.Header.Get("X-Resolution"); res != "" {
		parts := strings.Split(res, "x")
		if len(parts) == 2 {
			if w, _ := strconv.Atoi(strings.TrimSpace(parts[0])); w > 0 {
				caps.MaxWidth = w
			}
			if h, _ := strconv.Atoi(strings.TrimSpace(parts[1])); h > 0 {
				caps.MaxHeight = h
			}
		}
	}

	// Max bitrate: X-Max-Bitrate: 20000000 (bps)
	if bitrate := r.Header.Get("X-Max-Bitrate"); bitrate != "" {
		if br, err := strconv.ParseInt(bitrate, 10, 64); err == nil {
			caps.MaxBitrate = br
		}
	}

	// DRM support: X-DRM: widevine,playready,fairplay
	if drm := r.Header.Get("X-DRM"); drm != "" {
		caps.DRMSupport = &probes.DRMSupported{
			Supported: true,
			Systems:  strings.Split(strings.ToLower(drm), ","),
		}
	}

	// App version: X-App-Version: 1.2.3
	caps.AppVersion = r.Header.Get("X-App-Version")

	// Device name: X-Device-Name: Chrome on Windows
	caps.DeviceName = r.Header.Get("X-Device-Name")
}

// extractProbeBody extracts full probe report from request body
func (w *WebClientAdapter) extractProbeBody(r *http.Request, caps *probes.DeviceCapabilities) error {
	// Don't consume the body if it needs to be read again
	if r.Body != nil {
		var report probes.ProbeReport
		if err := json.NewDecoder(r.Body).Decode(&report); err == nil {
			// Merge report capabilities
			if report.DeviceCapabilities.DeviceID != "" {
				caps.DeviceID = report.DeviceCapabilities.DeviceID
			}
			if report.DeviceCapabilities.Platform != "" {
				caps.Platform = report.DeviceCapabilities.Platform
			}
			if len(report.DeviceCapabilities.VideoCodecs) > 0 {
				caps.VideoCodecs = report.DeviceCapabilities.VideoCodecs
			}
			if len(report.DeviceCapabilities.AudioCodecs) > 0 {
				caps.AudioCodecs = report.DeviceCapabilities.AudioCodecs
			}
			if report.DeviceCapabilities.MaxWidth > 0 {
				caps.MaxWidth = report.DeviceCapabilities.MaxWidth
			}
			if report.DeviceCapabilities.MaxHeight > 0 {
				caps.MaxHeight = report.DeviceCapabilities.MaxHeight
			}
			if report.DeviceCapabilities.MaxBitrate > 0 {
				caps.MaxBitrate = report.DeviceCapabilities.MaxBitrate
			}
			caps.SupportsHDR = caps.SupportsHDR || report.DeviceCapabilities.SupportsHDR
			caps.SupportsDolbyVision = caps.SupportsDolbyVision || report.DeviceCapabilities.SupportsDolbyVision
			return nil
		}
	}
	return nil
}

// applyBrowserDefaults sets default capabilities based on browser type
func (w *WebClientAdapter) applyBrowserDefaults(caps *probes.DeviceCapabilities) {
	// Detect browser type for default codec selection
	browser := caps.Identity.Platform.WebBrowser
	if browser == "" {
		browser = caps.Platform
	}

	// Set default codecs if not specified
	if len(caps.VideoCodecs) == 0 {
		switch browser {
		case "chrome", "edge":
			caps.VideoCodecs = []string{"h264", "vp8", "vp9", "av1"}
			caps.AudioCodecs = []string{"opus", "vorbis", "aac"}
		case "firefox":
			caps.VideoCodecs = []string{"h264", "vp8", "vp9"}
			caps.AudioCodecs = []string{"opus", "vorbis", "aac"}
		case "safari":
			caps.VideoCodecs = []string{"h264", "hevc"}
			caps.AudioCodecs = []string{"aac", "alac", "opus"}
		default:
			caps.VideoCodecs = []string{"h264"}
			caps.AudioCodecs = []string{"aac", "mp3"}
		}
	}

	// Set default containers if not specified
	if len(caps.ContainerFormats) == 0 {
		caps.ContainerFormats = []string{"mp4", "webm"}
	}

	// Set default subtitle formats
	if len(caps.SubtitleFormats) == 0 {
		caps.SubtitleFormats = []string{"vtt", "srt"}
	}

	// Set default resolution if not specified
	if caps.MaxWidth == 0 || caps.MaxHeight == 0 {
		caps.MaxWidth = 1920
		caps.MaxHeight = 1080
	}

	// Set default bitrate if not specified
	if caps.MaxBitrate == 0 {
		caps.MaxBitrate = 20_000_000 // 20 Mbps
	}
}

// calculateTrustScore adjusts trust based on capability consensus
func (w *WebClientAdapter) calculateTrustScore(caps *probes.DeviceCapabilities) {
	trust := w.baseTrust

	// Higher trust for full probe reports
	if len(caps.VideoCodecs) > 2 {
		trust += 0.10
	}

	// Higher trust for specific codec info
	if contains(caps.VideoCodecs, "av1") || contains(caps.VideoCodecs, "hevc") {
		trust += 0.05
	}

	// Higher trust for HDR info
	if caps.SupportsHDR || caps.SupportsDolbyVision {
		trust += 0.05
	}

	// Higher trust for specific resolution (not just "unknown")
	if caps.MaxWidth >= 3840 {
		trust += 0.05
	} else if caps.MaxWidth >= 1920 {
		trust += 0.02
	}

	// Higher trust for explicit DRM support (suggests professional app)
	if caps.DRMSupport != nil {
		trust += 0.05
	}

	// Cap at 0.90 for browser-reported capabilities
	if trust > 0.90 {
		trust = 0.90
	}

	caps.TrustScore = trust
	caps.TrustLevel = probes.TrustLevelFromScore(trust)
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return true
		}
	}
	return false
}
