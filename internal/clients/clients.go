// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package clients

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/tenkile/tenkile/internal/clients/android"
	"github.com/tenkile/tenkile/internal/clients/apple"
	"github.com/tenkile/tenkile/internal/probes"
)

// ErrNoAdapterFound is returned when no suitable adapter is found for a request
var ErrNoAdapterFound = errors.New("no client adapter found for request")

// Global registry for client adapters
var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// Registry manages platform-specific client adapters
type Registry struct {
	detector  *Detector
	adapters  map[string]ClientAdapter
	mu        sync.RWMutex
	defaults  []ClientAdapter // Ordered by priority
}

// GetRegistry returns the global adapter registry (singleton)
func GetRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// NewRegistry creates a new adapter registry with default adapters
func NewRegistry() *Registry {
	r := &Registry{
		detector: NewDetector(),
		adapters: make(map[string]ClientAdapter),
	}

	// Register default adapters
	// Web client adapter
	r.RegisterAdapter(NewWebClientAdapter())

	// Android adapter
	r.RegisterAdapter(android.NewAndroidAdapter())

	// Apple adapters (iOS, tvOS)
	r.RegisterAdapter(apple.NewIOSAdapter())
	r.RegisterAdapter(apple.NewTVOSAdapter())

	slog.Debug("Client registry initialized with default adapters")
	return r
}

// RegisterAdapter registers a platform-specific adapter
func (r *Registry) RegisterAdapter(adapter ClientAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.adapters[adapter.ClientType()] = adapter
	r.detector.RegisterAdapter(adapter)

	// Add to defaults list for priority ordering
	r.defaults = append(r.defaults, adapter)

	slog.Debug("Registered client adapter", "type", adapter.ClientType(), "source", adapter.SourceID())
}

// GetAdapter returns the adapter for a specific platform
func (r *Registry) GetAdapter(platform string) ClientAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[platform]
}

// DetectClient detects the appropriate adapter for the request
func (r *Registry) DetectClient(req *http.Request) ClientAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.detector.Detect(req)
}

// DetectPlatform returns the detected platform without returning an adapter
func (r *Registry) DetectPlatform(req *http.Request) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.detector.detectPlatform(req)
}

// DetectAndExtract combines detection and capability extraction
func (r *Registry) DetectAndExtract(req *http.Request) (*DetectorResult, error) {
	adapter := r.DetectClient(req)
	if adapter == nil {
		return nil, ErrNoAdapterFound
	}

	caps, err := adapter.ExtractCapabilities(req)
	if err != nil {
		return nil, err
	}

	return &DetectorResult{
		Adapter:    adapter,
		Platform:   adapter.ClientType(),
		SourceID:   adapter.SourceID(),
		Capabilities: caps,
		TrustScore: adapter.TrustScore(),
	}, nil
}

// DetectorResult contains the result of platform detection
type DetectorResult struct {
	Adapter       ClientAdapter
	Platform     string
	SourceID     string
	Capabilities *probes.DeviceCapabilities
	TrustScore   float64
}

// SetDetector sets a custom detector (for testing)
func (r *Registry) SetDetector(d *Detector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detector = d
}

// GetDetector returns the current detector
func (r *Registry) GetDetector() *Detector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.detector
}

// ListAdapters returns all registered adapters
func (r *Registry) ListAdapters() []ClientAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ClientAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		result = append(result, adapter)
	}
	return result
}

// RegisterAdapter is a package-level function that registers an adapter with the global registry
func RegisterAdapter(adapter ClientAdapter) {
	GetRegistry().RegisterAdapter(adapter)
}

// DetectClient is a package-level function that detects the client from a request
func DetectClient(req *http.Request) ClientAdapter {
	return GetRegistry().DetectClient(req)
}

// DetectPlatform is a package-level function that detects the platform from a request
func DetectPlatform(req *http.Request) string {
	return GetRegistry().DetectPlatform(req)
}

// GetAdapter is a package-level function that returns an adapter for a platform
func GetAdapter(platform string) ClientAdapter {
	return GetRegistry().GetAdapter(platform)
}

// DetectAndExtract is a package-level convenience function
func DetectAndExtract(req *http.Request) (*DetectorResult, error) {
	return GetRegistry().DetectAndExtract(req)
}
