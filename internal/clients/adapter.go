// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package clients

import (
	"net/http"

	"github.com/tenkile/tenkile/internal/probes"
)

// ClientAdapter is implemented by platform-specific probe libraries.
// Each adapter extracts capabilities from its native APIs and maps them to
// Tenkile's DeviceCapabilities model.
type ClientAdapter interface {
	// ClientType returns the platform identifier (e.g., "web", "android", "appletvos").
	ClientType() string

	// SourceID returns a unique identifier for this adapter (for trust scoring).
	SourceID() string

	// ExtractCapabilities extracts device capabilities from the HTTP request.
	// Returns nil if this adapter cannot handle the request.
	ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error)

	// TrustScore returns the trust score for this adapter's capabilities.
	// Higher trust = more reliable capability reports.
	TrustScore() float64
}
