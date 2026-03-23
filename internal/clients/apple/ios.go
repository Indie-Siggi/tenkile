// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package apple

import (
	"net/http"

	"github.com/tenkile/tenkile/internal/probes"
)

// IOSAdapter handles iOS device capability reports.
// iOS apps use AVFoundation to enumerate supported codecs
// and report results via the probe API.
type IOSAdapter struct {
	*BaseAppleAdapter
}

// NewIOSAdapter creates a new iOS adapter
func NewIOSAdapter() *IOSAdapter {
	return &IOSAdapter{
		BaseAppleAdapter: NewBaseAppleAdapter("ios", 0.82),
	}
}

// SourceID returns "ios-avfoundation"
func (a *IOSAdapter) SourceID() string {
	return "ios-avfoundation"
}

// ExtractCapabilities extracts device capabilities from an iOS probe report
func (a *IOSAdapter) ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error) {
	caps := &probes.DeviceCapabilities{
		Platform: "ios",
	}

	// Extract common Apple capabilities
	a.extractCommonCapabilities(r, caps)

	// iOS-specific: Extract device model identifier
	if model := r.Header.Get("X-Apple-Model-ID"); model != "" {
		caps.Model = model
		// Update identity with model
		caps.Identity.Model = model
	}

	// iOS-specific: Extract screen info
	if screen := r.Header.Get("X-Apple-Screen"); screen != "" {
		// Parse screen resolution (e.g., "390x844" for iPhone 12)
	}

	// iOS-specific: Hardware codec enumeration via VideoToolbox
	// The app can report hardware-accelerated codec support
	if hwCodecs := r.Header.Get("X-Apple-HW-Codecs"); hwCodecs != "" {
		// Hardware codec support is already captured via codec headers
	}

	return caps, nil
}
