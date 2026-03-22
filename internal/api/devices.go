// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/pkg/codec"
)

// DeviceHandlers holds device-related API handlers
type DeviceHandlers struct {
	validator *probes.Validator
	cache     *probes.CapabilityCache
	curatedDB *probes.CuratedDatabase
}

// NewDeviceHandlers creates new device handlers
func NewDeviceHandlers(
	validator *probes.Validator,
	cache *probes.CapabilityCache,
	curatedDB *probes.CuratedDatabase,
) *DeviceHandlers {
	return &DeviceHandlers{
		validator: validator,
		cache:     cache,
		curatedDB: curatedDB,
	}
}

// ProbeReportRequest represents a probe report request
type ProbeReportRequest struct {
	Capabilities *probes.DeviceCapabilities `json:"capabilities"`
	Source       string                     `json:"source"`
	Version      string                     `json:"version"`
}

// ProbeReportResponse represents a probe report response
type ProbeReportResponse struct {
	Success    bool                     `json:"success"`
	DeviceID   string                   `json:"device_id,omitempty"`
	CacheHit   bool                     `json:"cache_hit,omitempty"`
	Validation *probes.ValidationResult `json:"validation,omitempty"`
	Message    string                   `json:"message,omitempty"`
}

// CapabilitiesRequest represents a capabilities lookup request
type CapabilitiesRequest struct {
	DeviceID string `json:"device_id"`
	DeviceHash string `json:"device_hash"`
}

// CapabilitiesResponse represents a capabilities lookup response
type CapabilitiesResponse struct {
	Found       bool                  `json:"found"`
	Capabilities *probes.DeviceCapabilities `json:"capabilities,omitempty"`
	Cached      bool                  `json:"cached,omitempty"`
	Source      string                `json:"source,omitempty"`
	LastUpdated string                `json:"last_updated,omitempty"`
}

// DeviceProfile represents a device quality profile
type DeviceProfile struct {
	DeviceID       string            `json:"device_id"`
	ProfileName    string            `json:"profile_name"`
	MaxResolution  string            `json:"max_resolution"`
	RecommendedCodecs []string       `json:"recommended_codecs"`
	TranscodeRequired bool           `json:"transcode_required"`
	Notes          string            `json:"notes,omitempty"`
}

// determineProfileName returns a quality profile name based on device capabilities.
func determineProfileName(caps *probes.DeviceCapabilities) string {
	if caps.MaxWidth >= 3840 && caps.SupportsHDR {
		return "ultra_hd_hdr"
	}
	if caps.MaxWidth >= 3840 {
		return "ultra_hd"
	}
	if caps.MaxWidth >= 1920 && caps.SupportsHDR {
		return "full_hd_hdr"
	}
	if caps.MaxWidth >= 1920 {
		return "full_hd"
	}
	if caps.MaxWidth >= 1280 {
		return "hd"
	}
	return "sd"
}

// handleProbeReport handles probe report submissions
func (h *DeviceHandlers) handleProbeReport(w http.ResponseWriter, r *http.Request) {
	var req ProbeReportRequest

	// Parse request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	// Validate capabilities
	if req.Capabilities == nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_capabilities",
			Message: "Capabilities are required",
		})
		return
	}

	// Set timestamp if not provided
	if req.Capabilities.Timestamp == "" {
		req.Capabilities.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Validate the capabilities
	validation := h.validator.ValidateCapabilities(req.Capabilities)

	// Check cache for existing capabilities
	var cacheHit bool

	if _, found := h.cache.Get(req.Capabilities.DeviceID); found {
		cacheHit = true
	}

	// Store in cache
	source := req.Source
	if source == "" {
		source = "probe"
	}
	if err := h.cache.Set(req.Capabilities.DeviceID, req.Capabilities, source); err != nil {
		slog.Warn("Failed to cache capabilities", "error", err)
	}

	// Build response
	resp := ProbeReportResponse{
		Success:    validation.IsValid,
		DeviceID:   req.Capabilities.DeviceID,
		CacheHit:   cacheHit,
		Validation: validation,
	}

	if !validation.IsValid {
		resp.Message = "Capabilities have validation errors"
	} else if validation.AnomalyDetected {
		resp.Message = "Anomaly detected in capabilities"
	} else {
		resp.Message = "Capabilities recorded successfully"
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetCapabilities handles capability lookups
func (h *DeviceHandlers) handleGetCapabilities(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	deviceHash := r.URL.Query().Get("device_hash")

	if deviceID == "" && deviceHash == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_identifier",
			Message: "device_id or device_hash is required",
		})
		return
	}

	var caps *probes.DeviceCapabilities
	var found bool
	var cached bool

	// Try cache first
	if deviceID != "" {
		caps, found = h.cache.Get(deviceID)
		if found {
			cached = true
		}
	}

	// Try curated database
	if !found && deviceHash != "" {
		if device, ok := h.curatedDB.GetByDeviceHash(deviceHash); ok {
			caps = device.Capabilities
			found = true
		}
	}

	// Try curated database by device ID
	if !found && deviceID != "" {
		if device, ok := h.curatedDB.GetByID(deviceID); ok {
			caps = device.Capabilities
			found = true
		}
	}

	if !found {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "No capabilities found for the specified device",
		})
		return
	}

	// Update access count if from cache
	if cached {
		_ = h.cache.UpdateAccessCount(deviceID)
	}

	resp := CapabilitiesResponse{
		Found:       true,
		Capabilities: caps,
		Cached:      cached,
		Source:      "cache",
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleValidateCapabilities handles capability validation
func (h *DeviceHandlers) handleValidateCapabilities(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Capabilities *probes.DeviceCapabilities `json:"capabilities"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if req.Capabilities == nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_capabilities",
			Message: "Capabilities are required",
		})
		return
	}

	validation := h.validator.ValidateCapabilities(req.Capabilities)
	validation.SortErrors()

	RespondJSON(w, http.StatusOK, validation)
}

// handleGetDeviceProfile handles device profile requests
func (h *DeviceHandlers) handleGetDeviceProfile(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	// Try cache first
	caps, found := h.cache.Get(deviceID)
	if !found {
		// Try curated database
		if device, ok := h.curatedDB.GetByID(deviceID); ok {
			caps = device.Capabilities
			found = true
		}
	}

	if !found {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Device not found",
		})
		return
	}

	// Determine profile based on capabilities
	maxResolution := caps.MaxResolution()
	profileName := determineProfileName(caps)

	profile := DeviceProfile{
		DeviceID:          deviceID,
		ProfileName:       profileName,
		MaxResolution:     maxResolution,
		RecommendedCodecs: caps.VideoCodecs,
		TranscodeRequired: len(caps.VideoCodecs) == 0,
	}

	RespondJSON(w, http.StatusOK, profile)
}

// handleSearchDevices handles device search requests
func (h *DeviceHandlers) handleSearchDevices(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	manufacturer := r.URL.Query().Get("manufacturer")
	model := r.URL.Query().Get("model")
	videoCodec := r.URL.Query().Get("video_codec")
	audioCodec := r.URL.Query().Get("audio_codec")
	verifiedOnly := r.URL.Query().Get("verified") == "true"

	criteria := probes.SearchCriteria{
		Platform:     platform,
		Manufacturer: manufacturer,
		Model:        model,
		VideoCodec:   videoCodec,
		AudioCodec:   audioCodec,
		VerifiedOnly: verifiedOnly,
	}

	results := h.curatedDB.Search(criteria)

	// Limit results
	maxResults := 100
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"count":  len(results),
		"limit":  maxResults,
		"devices": results,
	})
}

// handleGetCuratedDevices handles curated device list requests
func (h *DeviceHandlers) handleGetCuratedDevices(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	limit := 50

	if platform != "" {
		devices := h.curatedDB.GetByPlatform(platform)
		if len(devices) > limit {
			devices = devices[:limit]
		}

		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"count":   len(devices),
			"platform": platform,
			"devices":  devices,
		})
		return
	}

	// Return all devices
	stats := h.curatedDB.GetStats()
	allDevices := h.curatedDB.GetAll()
	if len(allDevices) > limit {
		allDevices = allDevices[:limit]
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"count":    stats.TotalDevices,
		"limit":    limit,
		"devices":  allDevices,
		"stats":    stats,
	})
}

// handleGetDevicesByPlatform handles platform-specific device requests
func (h *DeviceHandlers) handleGetDevicesByPlatform(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")

	if platform == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_platform",
			Message: "Platform is required",
		})
		return
	}

	devices := h.curatedDB.GetByPlatform(platform)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"platform": platform,
		"count":    len(devices),
		"devices":  devices,
	})
}

// handleGetDevicesByCodec handles codec-specific device requests
func (h *DeviceHandlers) handleGetDevicesByCodec(w http.ResponseWriter, r *http.Request) {
	codecName := r.PathValue("codec")

	if codecName == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_codec",
			Message: "Codec is required",
		})
		return
	}

	// Validate codec name
	if !codec.IsVideoCodec(codecName) && !codec.IsAudioCodec(codecName) {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_codec",
			Message: "Invalid codec name",
		})
		return
	}

	// Get from cache
	caps, err := h.cache.GetCapabilitiesByCodec(codecName)
	if err != nil {
		slog.Warn("Failed to get capabilities by codec from cache", "error", err)
		caps = []*probes.DeviceCapabilities{}
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"codec":    codecName,
		"count":    len(caps),
		"devices":  caps,
	})
}


