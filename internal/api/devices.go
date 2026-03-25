// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/pkg/codec"
)

// DeviceHandlers holds device-related API handlers
type DeviceHandlers struct {
	validator        *probes.Validator
	cache           *probes.CapabilityCache
	curatedDB       *probes.CuratedDatabase
	feedbackManager *probes.FeedbackManager
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

// SetFeedbackManager sets the feedback manager for playback tracking
func (h *DeviceHandlers) SetFeedbackManager(fm *probes.FeedbackManager) {
	h.feedbackManager = fm
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

	if deviceID != "" && !isValidDeviceID(deviceID) {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_device_id",
			Message: "Device ID format is invalid",
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

	if !isValidDeviceID(platform) {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_platform",
			Message: "Platform format is invalid",
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

// --- Phase 3.2: Playback Feedback Loop Handlers ---

// DevicePlaybackFeedbackRequest represents a playback feedback request for a specific device
type DevicePlaybackFeedbackRequest struct {
	MediaID        string  `json:"media_id"`
	Outcome        string  `json:"outcome"`
	ErrorCode      string  `json:"error_code,omitempty"`
	ErrorMessage   string  `json:"error_message,omitempty"`
	Duration       float64 `json:"duration_seconds,omitempty"`
	BufferDuration float64 `json:"buffer_duration_seconds,omitempty"`
	NetworkQuality string  `json:"network_quality,omitempty"`
	VideoCodec     string  `json:"video_codec,omitempty"`
	AudioCodec     string  `json:"audio_codec,omitempty"`
	Container      string  `json:"container,omitempty"`
	Resolution     string  `json:"resolution,omitempty"`
	Bitrate        int64   `json:"bitrate,omitempty"`
}

// PlaybackFeedbackResponse represents a playback feedback response
type PlaybackFeedbackResponse struct {
	Success        bool    `json:"success"`
	Recorded       bool    `json:"recorded"`
	TrustDelta     float64 `json:"trust_delta,omitempty"`
	NeedsReProbe   bool    `json:"needs_reprobe,omitempty"`
	ReProbeReason  string  `json:"reprobe_reason,omitempty"`
	Message        string  `json:"message,omitempty"`
}

// handlePlaybackFeedback handles playback feedback submissions
func (h *DeviceHandlers) handlePlaybackFeedback(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	if !isValidDeviceID(deviceID) {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_device_id",
			Message: "Device ID format is invalid",
		})
		return
	}

	if h.feedbackManager == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "feedback_not_enabled",
			Message: "Playback feedback is not enabled",
		})
		return
	}

	var req DevicePlaybackFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	// Parse outcome
	outcome := probes.ParseOutcomeFromString(req.Outcome)
	if outcome == probes.OutcomeUnknown && req.Outcome != "" && req.Outcome != "unknown" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_outcome",
			Message: "Invalid outcome value. Valid values: success, network_error, codec_error, decoding_failed, renderer_crash, unsupported_format, timeout, buffering",
		})
		return
	}

	// Create feedback
	feedback := probes.PlaybackFeedback{
		DeviceID:      deviceID,
		MediaID:       req.MediaID,
		Outcome:       outcome,
		ErrorCode:     req.ErrorCode,
		ErrorMessage:  req.ErrorMessage,
		Timestamp:     time.Now(),
		NetworkQuality: req.NetworkQuality,
		VideoCodec:    req.VideoCodec,
		AudioCodec:    req.AudioCodec,
		Container:     req.Container,
		Resolution:    req.Resolution,
		Bitrate:       req.Bitrate,
	}

	if req.Duration > 0 {
		feedback.Duration = time.Duration(req.Duration * float64(time.Second))
	}
	if req.BufferDuration > 0 {
		feedback.BufferDuration = time.Duration(req.BufferDuration * float64(time.Second))
	}

	// Calculate trust delta before recording
	trustDelta := h.feedbackManager.CalculateTrustDelta(outcome)

	// Record feedback
	if outcome == probes.OutcomeSuccess {
		h.feedbackManager.RecordSuccess(feedback)
	} else {
		h.feedbackManager.RecordFailure(feedback)
	}

	// Check if re-probe is needed
	needsReProbe, reProbeReason := h.feedbackManager.ShouldReProbe(deviceID)

	// Record metrics
	metrics := probes.GetGlobalPlaybackMetrics()
	metrics.RecordPlayback(deviceID, outcome)
	metrics.RecordFeedbackLatency(time.Duration(0)) // Latency tracking would be done at middleware level
	if trustDelta != 0 {
		metrics.RecordTrustScore(deviceID, h.feedbackManager.GetTrustAdjustment(deviceID))
	}
	if needsReProbe {
		metrics.RecordReProbe(deviceID, reProbeReason)
	}

	resp := PlaybackFeedbackResponse{
		Success:      true,
		Recorded:     true,
		TrustDelta:   trustDelta,
		NeedsReProbe: needsReProbe,
		ReProbeReason: reProbeReason,
		Message:      "Feedback recorded successfully",
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetPlaybackStats handles playback statistics requests
func (h *DeviceHandlers) handleGetPlaybackStats(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	if h.feedbackManager == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "feedback_not_enabled",
			Message: "Playback feedback is not enabled",
		})
		return
	}

	stats := h.feedbackManager.GetPlaybackStats(deviceID)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"device_id": deviceID,
		"stats":     stats,
	})
}

// handleGetReliableCodecs handles reliable codecs requests
func (h *DeviceHandlers) handleGetReliableCodecs(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	if h.feedbackManager == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "feedback_not_enabled",
			Message: "Playback feedback is not enabled",
		})
		return
	}

	// Get minimum success rate from query parameter (default 0.8)
	minSuccessRate := 0.8
	if rate := r.URL.Query().Get("min_rate"); rate != "" {
		if f, err := strconv.ParseFloat(rate, 64); err == nil {
			minSuccessRate = f
		}
	}

	reliableCodecs := h.feedbackManager.GetReliableCodecs(deviceID, minSuccessRate)

	// Get detailed codec stats
	stats := h.feedbackManager.GetPlaybackStats(deviceID)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":      deviceID,
		"reliable_codecs": reliableCodecs,
		"min_success_rate": minSuccessRate,
		"codec_stats":    stats.CodecStats,
	})
}

// handleReProbeDevice handles re-probe trigger requests
func (h *DeviceHandlers) handleReProbeDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	// Get current device info
	caps, found := h.cache.Get(deviceID)
	if !found {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "device_not_found",
			Message: "Device not found in cache",
		})
		return
	}

	// Clear from cache to force re-probe
	h.cache.Delete(deviceID)

	// Reset feedback trust adjustment
	if h.feedbackManager != nil {
		h.feedbackManager.ResetTrustAdjustment(deviceID)
	}

	// Record that a re-probe was triggered
	metrics := probes.GetGlobalPlaybackMetrics()
	metrics.RecordReProbe(deviceID, "manual_trigger")

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": deviceID,
		"message":   "Re-probe triggered. Device has been cleared from cache.",
		"previous_caps": map[string]interface{}{
			"platform":     caps.Platform,
			"manufacturer": caps.Manufacturer,
			"model":        caps.Model,
			"video_codecs": caps.VideoCodecs,
			"audio_codecs": caps.AudioCodecs,
		},
	})
}

// handleGetFeedbackMetrics handles metrics export requests
func (h *DeviceHandlers) handleGetFeedbackMetrics(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	format := r.URL.Query().Get("format")

	if h.feedbackManager == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "feedback_not_enabled",
			Message: "Playback feedback is not enabled",
		})
		return
	}

	metrics := probes.GetGlobalPlaybackMetrics()

	// Check if Prometheus format requested
	if format == "prometheus" {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metrics.ExportPrometheusFormat()))
		return
	}

	// Default JSON format
	var response map[string]interface{}

	if deviceID != "" {
		// Get device-specific metrics
		stats := h.feedbackManager.GetPlaybackStats(deviceID)
		trustScore := h.feedbackManager.GetTrustAdjustment(deviceID)

		response = map[string]interface{}{
			"device_id":    deviceID,
			"stats":       stats,
			"trust_score":  trustScore,
		}
	} else {
		// Get global metrics
		response = map[string]interface{}{
			"metrics":    metrics.ExportMetrics(),
			"global_stats": h.feedbackManager.GetGlobalStats(),
		}
	}

	RespondJSON(w, http.StatusOK, response)
}

// handleGetTrustReport handles trust report requests
func (h *DeviceHandlers) handleGetTrustReport(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	if h.feedbackManager == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "feedback_not_enabled",
			Message: "Playback feedback is not enabled",
		})
		return
	}

	stats := h.feedbackManager.GetPlaybackStats(deviceID)
	adjustment := h.feedbackManager.GetTrustAdjustment(deviceID)
	reliableCodecs := h.feedbackManager.GetReliableCodecs(deviceID, 0.8)

	// Determine trust level
	trustLevel := "low"
	effectiveScore := adjustment
	if effectiveScore >= 0.8 {
		trustLevel = "very_high"
	} else if effectiveScore >= 0.7 {
		trustLevel = "high"
	} else if effectiveScore >= 0.5 {
		trustLevel = "medium"
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"device_id":           deviceID,
		"effective_trust":    effectiveScore,
		"trust_level":        trustLevel,
		"feedback_adjustment": adjustment,
		"needs_reprobe":      stats.NeedsReProbe,
		"reprobe_reason":     stats.ReProbeReason,
		"reliable_codecs":    reliableCodecs,
		"playback_stats":     stats,
	})
}


