// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package apple

import (
	"net/http"

	"github.com/tenkile/tenkile/internal/probes"
)

// TVOSAdapter handles tvOS device capability reports.
// tvOS apps use AVFoundation and VideoToolbox to enumerate supported codecs
// and report results via the probe API.
type TVOSAdapter struct {
	*BaseAppleAdapter
}

// NewTVOSAdapter creates a new tvOS adapter
func NewTVOSAdapter() *TVOSAdapter {
	return &TVOSAdapter{
		BaseAppleAdapter: NewBaseAppleAdapter("appletvos", 0.88),
	}
}

// SourceID returns "tvos-avfoundation"
func (a *TVOSAdapter) SourceID() string {
	return "tvos-avfoundation"
}

// ExtractCapabilities extracts device capabilities from a tvOS probe report
func (a *TVOSAdapter) ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error) {
	caps := &probes.DeviceCapabilities{
		Platform: "appletvos",
	}

	// Extract common Apple capabilities
	a.extractCommonCapabilities(r, caps)

	// tvOS-specific: Device generation
	if gen := r.Header.Get("X-Apple-TV-Generation"); gen != "" {
		// Store generation info for capability inference
	}

	// tvOS-specific: 4K support is standard on newer models
	if caps.MaxWidth == 0 && caps.MaxHeight == 0 {
		// Default to 1920x1080 for older Apple TV, 3840x2160 for newer
		caps.MaxWidth = 1920
		caps.MaxHeight = 1080
	}

	// tvOS-specific: Dolby Vision is widely supported
	if caps.SupportsDolbyVision {
		caps.SupportsHDR = true
	}

	// tvOS-specific: Higher default bitrate for TV streaming
	if caps.MaxBitrate == 0 {
		caps.MaxBitrate = 100_000_000 // 100 Mbps
	}

	return caps, nil
}

// getPlatformType overrides for tvOS
func (a *TVOSAdapter) getPlatformType() string {
	return "smart_tv"
}
