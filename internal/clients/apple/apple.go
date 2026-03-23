// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package apple

import (
	"net/http"
	"strings"

	"github.com/tenkile/tenkile/internal/probes"
)

// ClientAdapter is implemented by platform-specific probe libraries.
type ClientAdapter interface {
	ClientType() string
	SourceID() string
	ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error)
	TrustScore() float64
}

// BaseAppleAdapter provides common functionality for Apple platforms
type BaseAppleAdapter struct {
	platform  string
	baseTrust float64
}

// NewBaseAppleAdapter creates a base Apple adapter
func NewBaseAppleAdapter(platform string, trust float64) *BaseAppleAdapter {
	return &BaseAppleAdapter{
		platform:  platform,
		baseTrust: trust,
	}
}

// ClientType returns the platform identifier
func (a *BaseAppleAdapter) ClientType() string {
	return a.platform
}

// TrustScore returns the base trust score
func (a *BaseAppleAdapter) TrustScore() float64 {
	return a.baseTrust
}

// extractCommonCapabilities extracts capabilities common to all Apple platforms
func (a *BaseAppleAdapter) extractCommonCapabilities(r *http.Request, caps *probes.DeviceCapabilities) {
	// Extract device identity
	caps.DeviceID = r.Header.Get("X-Device-ID")
	if caps.DeviceID == "" {
		caps.DeviceID = r.Header.Get("X-DeviceId")
	}
	caps.UserAgent = r.UserAgent()

	// Extract Apple-specific headers
	caps.Manufacturer = "Apple"
	caps.Model = r.Header.Get("X-Apple-Model")
	caps.OSVersion = r.Header.Get("X-Apple-Version")
	caps.AppVersion = r.Header.Get("X-App-Version")
	caps.DeviceName = r.Header.Get("X-Device-Name")

	// Set platform type
	caps.Identity.Platform.Type = a.getPlatformType()
	caps.Identity.Platform.Name = a.getPlatformName()
	caps.Identity.Platform.OS = a.getOSName()
	caps.Identity.Platform.OSVersion = caps.OSVersion
	caps.Identity.Manufacturer = caps.Manufacturer
	caps.Identity.Model = caps.Model

	// Extract codec information from headers
	a.extractCodecHeaders(r, caps)

	// Try to extract full probe report from request body
	if r.ContentLength > 0 && r.ContentLength < 1<<20 {
		a.extractProbeBody(r, caps)
	}

	// Apply platform defaults
	a.applyPlatformDefaults(caps)

	// Calculate trust score
	a.calculateTrustScore(caps)
}

// getPlatformType returns the platform type string
func (a *BaseAppleAdapter) getPlatformType() string {
	switch a.platform {
	case "ios", "tvos":
		return "mobile"
	case "appletvos":
		return "smart_tv"
	case "macos":
		return "desktop"
	default:
		return "mobile"
	}
}

// getPlatformName returns the display name of the platform
func (a *BaseAppleAdapter) getPlatformName() string {
	switch a.platform {
	case "ios":
		return "iOS"
	case "tvos", "appletvos":
		return "Apple TV"
	case "macos":
		return "macOS"
	default:
		return "Apple"
	}
}

// getOSName returns the OS name
func (a *BaseAppleAdapter) getOSName() string {
	switch a.platform {
	case "ios":
		return "iOS"
	case "tvos", "appletvos":
		return "tvOS"
	case "macos":
		return "macOS"
	default:
		return "iOS"
	}
}

// extractCodecHeaders extracts codec support from Apple-specific headers
func (a *BaseAppleAdapter) extractCodecHeaders(r *http.Request, caps *probes.DeviceCapabilities) {
	// Video codecs: X-Apple-Video-Codecs: hvc1,avc1,av01
	if codecs := r.Header.Get("X-Apple-Video-Codecs"); codecs != "" {
		caps.VideoCodecs = parseCodecList(codecs, true)
	}

	// Audio codecs: X-Apple-Audio-Codecs: aac,alac,ac3,eac3
	if codecs := r.Header.Get("X-Apple-Audio-Codecs"); codecs != "" {
		caps.AudioCodecs = parseCodecList(codecs, false)
	}

	// HDR types: X-Apple-HDR: hdr10,dolby-vision,hdr10plus
	if hdr := r.Header.Get("X-Apple-HDR"); hdr != "" {
		hdrs := strings.Split(strings.ToLower(hdr), ",")
		for _, h := range hdrs {
			h = strings.TrimSpace(h)
			switch h {
			case "hdr10":
				caps.SupportsHDR = true
			case "dolby-vision", "dv":
				caps.SupportsDolbyVision = true
			case "hdr10plus":
				caps.Supports10Bit = true
			}
		}
	}

	// DRM systems: X-Apple-DRM: fairplay
	if drm := r.Header.Get("X-Apple-DRM"); drm != "" {
		systems := strings.Split(strings.ToLower(drm), ",")
		for i := range systems {
			systems[i] = strings.TrimSpace(systems[i])
		}
		caps.DRMSupport = &probes.DRMSupported{
			Supported: true,
			Systems:   systems,
		}
	}

	// Max resolution: X-Apple-Resolution: 3840x2160
	if res := r.Header.Get("X-Apple-Resolution"); res != "" {
		parts := strings.Split(res, "x")
		if len(parts) == 2 {
			// Parse dimensions
			caps.MaxWidth = parseInt(parts[0])
			caps.MaxHeight = parseInt(parts[1])
		}
	}

	// Max bitrate: X-Apple-Max-Bitrate: 50000000
	if bitrate := r.Header.Get("X-Apple-Max-Bitrate"); bitrate != "" {
		caps.MaxBitrate = parseInt64(bitrate)
	}

	// Container formats: X-Apple-Containers: mov,mp4,m4v
	if containers := r.Header.Get("X-Apple-Containers"); containers != "" {
		caps.ContainerFormats = parseContainerList(containers)
	}
}

// extractProbeBody extracts full probe report from JSON body
func (a *BaseAppleAdapter) extractProbeBody(r *http.Request, caps *probes.DeviceCapabilities) {
	// Implementation depends on Apple's specific JSON format
	// This is a placeholder for when the iOS/tvOS apps implement the probe API
}

// applyPlatformDefaults sets default capabilities for Apple platforms
func (a *BaseAppleAdapter) applyPlatformDefaults(caps *probes.DeviceCapabilities) {
	// Set default video codecs if not specified
	if len(caps.VideoCodecs) == 0 {
		caps.VideoCodecs = a.defaultVideoCodecs()
	}

	// Set default audio codecs if not specified
	if len(caps.AudioCodecs) == 0 {
		caps.AudioCodecs = a.defaultAudioCodecs()
	}

	// Set default containers if not specified
	if len(caps.ContainerFormats) == 0 {
		caps.ContainerFormats = a.defaultContainers()
	}

	// Set default subtitle formats
	if len(caps.SubtitleFormats) == 0 {
		caps.SubtitleFormats = []string{"srt", "vtt", "ssa", "ass"}
	}

	// Set default resolution if not specified
	if caps.MaxWidth == 0 || caps.MaxHeight == 0 {
		caps.MaxWidth = 1920
		caps.MaxHeight = 1080
	}

	// Set default bitrate if not specified
	if caps.MaxBitrate == 0 {
		caps.MaxBitrate = 50_000_000 // 50 Mbps
	}
}

// defaultVideoCodecs returns platform-specific default video codecs
func (a *BaseAppleAdapter) defaultVideoCodecs() []string {
	switch a.platform {
	case "ios":
		return []string{"h264", "hevc"}
	case "tvos", "appletvos":
		return []string{"h264", "hevc"}
	case "macos":
		return []string{"h264", "hevc", "prores"}
	default:
		return []string{"h264"}
	}
}

// defaultAudioCodecs returns platform-specific default audio codecs
func (a *BaseAppleAdapter) defaultAudioCodecs() []string {
	switch a.platform {
	case "ios", "tvos", "appletvos":
		return []string{"aac", "alac", "ac3", "eac3"}
	case "macos":
		return []string{"aac", "alac", "mp3", "ac3", "eac3", "flac"}
	default:
		return []string{"aac", "alac"}
	}
}

// defaultContainers returns platform-specific default containers
func (a *BaseAppleAdapter) defaultContainers() []string {
	return []string{"mov", "mp4", "m4v"}
}

// calculateTrustScore adjusts trust based on Apple capability report
func (a *BaseAppleAdapter) calculateTrustScore(caps *probes.DeviceCapabilities) {
	trust := a.baseTrust

	// Higher trust for full codec enumeration
	if len(caps.VideoCodecs) >= 2 {
		trust += 0.05
	}

	// Higher trust for HEVC support (requires newer hardware)
	if contains(caps.VideoCodecs, "hevc") {
		trust += 0.05
	}

	// Higher trust for Dolby Vision (requires certified hardware)
	if caps.SupportsDolbyVision {
		trust += 0.05
	}

	// Higher trust for FairPlay DRM
	if caps.DRMSupport != nil && contains(caps.DRMSupport.Systems, "fairplay") {
		trust += 0.05
	}

	// Higher trust for 4K support
	if caps.MaxWidth >= 3840 {
		trust += 0.02
	}

	// Cap trust score
	if trust > 0.98 {
		trust = 0.98
	}

	caps.TrustScore = trust
	caps.TrustLevel = probes.TrustLevelFromScore(trust)
}

// ClientAdapter interface for platform adapters
var _ ClientAdapter = (*IOSAdapter)(nil)
var _ ClientAdapter = (*TVOSAdapter)(nil)
