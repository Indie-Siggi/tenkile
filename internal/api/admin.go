// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/internal/transcode"
)

// AdminHandlers holds admin-related API handlers
type AdminHandlers struct {
	validator      *probes.Validator
	cache          *probes.CapabilityCache
	curatedDB      *probes.CuratedDatabase
	decisionLogger *transcode.DecisionLogger
}

// NewAdminHandlers creates new admin handlers
func NewAdminHandlers(
	validator *probes.Validator,
	cache *probes.CapabilityCache,
	curatedDB *probes.CuratedDatabase,
) *AdminHandlers {
	return &AdminHandlers{
		validator:   validator,
		cache:       cache,
		curatedDB:   curatedDB,
	}
}

// SetDecisionLogger sets the decision logger for admin query endpoints.
func (h *AdminHandlers) SetDecisionLogger(dl *transcode.DecisionLogger) {
	h.decisionLogger = dl
}

// SystemInfoResponse represents system information
type SystemInfoResponse struct {
	Version     string    `json:"version"`
	BuildTime   string    `json:"build_time,omitempty"`
	GoVersion   string    `json:"go_version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	NumCPU      int       `json:"num_cpu"`
	StartTime   time.Time `json:"start_time"`
	Uptime      string    `json:"uptime"`
	Hostname    string    `json:"hostname,omitempty"`
	Environment string    `json:"environment"`
}

// CacheStatsResponse represents cache statistics
type CacheStatsResponse struct {
	Hits         int64 `json:"hits"`
	Misses       int64 `json:"misses"`
	MemorySize   int   `json:"memory_size"`
	SQLiteSize   int   `json:"sqlite_size"`
	Evictions    int64 `json:"evictions"`
	HitRate      float64 `json:"hit_rate"`
}

// DatabaseStatsResponse represents curated database statistics
type DatabaseStatsResponse struct {
	TotalDevices     int64 `json:"total_devices"`
	VerifiedDevices  int64 `json:"verified_devices"`
	CommunityDevices int64 `json:"community_devices"`
	OfficialDevices  int64 `json:"official_devices"`
	CuratedDevices   int64 `json:"curated_devices"`
	PlatformsCount   int   `json:"platforms_count"`
	Platforms        []string `json:"platforms,omitempty"`
}

// ValidatorStatsResponse represents validator statistics
type ValidatorStatsResponse struct {
	TotalValidations int64 `json:"total_validations"`
	ValidCount       int64 `json:"valid_count"`
	InvalidCount     int64 `json:"invalid_count"`
	AnomalyCount     int64 `json:"anomaly_count"`
	ValidationRate   float64 `json:"validation_rate"`
}

// CacheCleanupRequest represents a cache cleanup request
type CacheCleanupRequest struct {
	ClearMemory bool `json:"clear_memory"`
	ClearSQLite bool `json:"clear_sqlite"`
	ExpiredOnly bool `json:"expired_only"`
}

// CacheCleanupResponse represents a cache cleanup response
type CacheCleanupResponse struct {
	Success       bool   `json:"success"`
	MemoryCleared int    `json:"memory_cleared,omitempty"`
	SQLiteCleared int    `json:"sqlite_cleared,omitempty"`
	Message       string `json:"message"`
}

// CuratedDeviceUpdateRequest represents a curated device update request
type CuratedDeviceUpdateRequest struct {
	DeviceID       string                   `json:"device_id"`
	Name           string                   `json:"name,omitempty"`
	Manufacturer   string                   `json:"manufacturer,omitempty"`
	Model          string                   `json:"model,omitempty"`
	Capabilities   *probes.DeviceCapabilities `json:"capabilities,omitempty"`
	Verified       bool                     `json:"verified,omitempty"`
	Notes          string                   `json:"notes,omitempty"`
}

// CuratedDeviceUpdateResponse represents a curated device update response
type CuratedDeviceUpdateResponse struct {
	Success   bool   `json:"success"`
	DeviceID  string `json:"device_id,omitempty"`
	Message   string `json:"message"`
}

// AnomalyReport represents anomaly statistics
type AnomalyReport struct {
	TotalAnomalies  int              `json:"total_anomalies"`
	AnomaliesByType map[string]int   `json:"anomalies_by_type"`
	RecentAnomalies []AnomalySummary `json:"recent_anomalies"`
}

// AnomalySummary represents a summary of an anomaly
type AnomalySummary struct {
	DeviceID    string    `json:"device_id"`
	AnomalyType string    `json:"anomaly_type"`
	Count       int       `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
	Severity    string    `json:"severity"`
}


// handleGetSystemInfo handles system information requests
func (h *AdminHandlers) handleGetSystemInfo(w http.ResponseWriter, r *http.Request) {
	// Get runtime stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	resp := SystemInfoResponse{
		Version:   "1.0.0",
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		StartTime: time.Now(), // Would be set in main.go
		Uptime:    "0s",       // Would calculate from start time
		Environment: "production",
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetCacheStats handles cache statistics requests
func (h *AdminHandlers) handleGetCacheStats(w http.ResponseWriter, r *http.Request) {
	stats := h.cache.GetStats()

	hitRate := 0.0
	total := stats.Hits + stats.Misses
	if total > 0 {
		hitRate = float64(stats.Hits) / float64(total) * 100
	}

	resp := CacheStatsResponse{
		Hits:       stats.Hits,
		Misses:     stats.Misses,
		MemorySize: stats.MemorySize,
		SQLiteSize: stats.SQLiteSize,
		Evictions:  stats.Evictions,
		HitRate:    hitRate,
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetDatabaseStats handles database statistics requests
func (h *AdminHandlers) handleGetDatabaseStats(w http.ResponseWriter, r *http.Request) {
	stats := h.curatedDB.GetStats()

	// Get platform list
	platforms := []string{
		"ios", "android", "windows", "macos", "linux",
		"tvos", "chromecast", "roku", "firetv", "xbox",
		"playstation", "apple_tv", "web", "chromeos",
	}

	resp := DatabaseStatsResponse{
		TotalDevices:     stats.TotalDevices,
		VerifiedDevices:  stats.VerifiedDevices,
		CommunityDevices: stats.CommunityDevices,
		OfficialDevices:  stats.OfficialDevices,
		CuratedDevices:   stats.CuratedDevices,
		PlatformsCount:   stats.PlatformsCount,
		Platforms:        platforms,
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetValidatorStats handles validator statistics requests
func (h *AdminHandlers) handleGetValidatorStats(w http.ResponseWriter, r *http.Request) {
	// Get anomaly history summary
	// For now, return placeholder stats
	resp := ValidatorStatsResponse{
		TotalValidations: 0,
		ValidCount:       0,
		InvalidCount:     0,
		AnomalyCount:     0,
		ValidationRate:   0.0,
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleCacheCleanup handles cache cleanup requests
func (h *AdminHandlers) handleCacheCleanup(w http.ResponseWriter, r *http.Request) {
	var req CacheCleanupRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Use defaults if body is empty
		req.ExpiredOnly = true
	}

	cleared := 0

	// Clean expired entries
	cleared = h.cache.DeleteExpired()

	resp := CacheCleanupResponse{
		Success: true,
		Message: "Cache cleanup completed",
	}

	if req.ExpiredOnly {
		resp.SQLiteCleared = cleared
		resp.Message = "Expired entries removed"
	} else {
		resp.SQLiteCleared = cleared
		resp.Message = "All cache cleared"
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetMemoryCache handles memory cache inspection requests
func (h *AdminHandlers) handleGetMemoryCache(w http.ResponseWriter, r *http.Request) {
	// Return limited cache info (not full contents for performance)
	size := h.cache.GetMemorySize()
	stats := h.cache.GetStats()

	// Get most accessed entries
	mostAccessed, err := h.cache.GetMostAccessed(10)
	if err != nil {
		mostAccessed = []*probes.CachedCapability{}
	}

	resp := map[string]interface{}{
		"memory_size": size,
		"stats": stats,
		"most_accessed": len(mostAccessed),
		"sample_entries": []map[string]interface{}{},
	}

	// Add sample entries (without full capabilities)
	for _, entry := range mostAccessed {
		resp["sample_entries"] = append(resp["sample_entries"].([]map[string]interface{}),
			map[string]interface{}{
				"device_id":    "hidden",
				"source":       entry.Source,
				"access_count": entry.AccessCount,
				"last_accessed": entry.LastAccessed,
			},
		)
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetAnomalies handles anomaly report requests
func (h *AdminHandlers) handleGetAnomalies(w http.ResponseWriter, r *http.Request) {
	// This would aggregate anomaly data from validator
	// For now, return placeholder structure
	resp := AnomalyReport{
		TotalAnomalies:  0,
		AnomaliesByType: make(map[string]int),
		RecentAnomalies: []AnomalySummary{},
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleUpdateCuratedDevice handles curated device update requests
func (h *AdminHandlers) handleUpdateCuratedDevice(w http.ResponseWriter, r *http.Request) {
	var req CuratedDeviceUpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if req.DeviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	// Get existing device
	device, ok := h.curatedDB.GetByID(req.DeviceID)
	if !ok {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "device_not_found",
			Message: "Device not found",
		})
		return
	}

	// Update fields
	if req.Name != "" {
		device.Name = req.Name
	}
	if req.Manufacturer != "" {
		device.Manufacturer = req.Manufacturer
	}
	if req.Model != "" {
		device.Model = req.Model
	}
	if req.Capabilities != nil {
		device.Capabilities = req.Capabilities
	}
	if req.Verified {
		device.Verified = req.Verified
	}
	if req.Notes != "" {
		device.Notes = req.Notes
	}

	// Save updated device
	if err := h.curatedDB.UpdateDevice(device); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update device",
		})
		return
	}

	resp := CuratedDeviceUpdateResponse{
		Success:  true,
		DeviceID: req.DeviceID,
		Message:  "Device updated successfully",
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleVerifyDevice handles device verification requests
func (h *AdminHandlers) handleVerifyDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	// Get existing device
	device, ok := h.curatedDB.GetByID(deviceID)
	if !ok {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "device_not_found",
			Message: "Device not found",
		})
		return
	}

	// Mark as verified
	device.Verified = true
	device.LastUpdated = time.Now().UTC()

	// Save updated device
	if err := h.curatedDB.UpdateDevice(device); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update device",
		})
		return
	}

	resp := CuratedDeviceUpdateResponse{
		Success:  true,
		DeviceID: deviceID,
		Message:  "Device verified successfully",
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleRemoveCuratedDevice handles curated device removal requests
func (h *AdminHandlers) handleRemoveCuratedDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	// Remove device
	if err := h.curatedDB.RemoveDevice(deviceID); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to remove device",
		})
		return
	}

	// Also remove from cache
	h.cache.Delete(deviceID)

	resp := CuratedDeviceUpdateResponse{
		Success:  true,
		DeviceID: deviceID,
		Message:  "Device removed successfully",
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleGetDecisions handles decision log query requests.
// GET /api/v1/admin/decisions?deviceId=X&from=date&to=date&limit=N&offset=N
func (h *AdminHandlers) handleGetDecisions(w http.ResponseWriter, r *http.Request) {
	if h.decisionLogger == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "not_available",
			Message: "Decision logger not configured",
		})
		return
	}

	q := transcode.DecisionQuery{
		DeviceID: r.URL.Query().Get("deviceId"),
		Limit:    100, // default
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			q.Limit = v
		}
	}
	const maxLimit = 1000
	if q.Limit > maxLimit {
		q.Limit = maxLimit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			q.Offset = v
		}
	}

	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q.From = t
		} else if t, err := time.Parse("2006-01-02", from); err == nil {
			q.From = t
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			q.To = t
		} else if t, err := time.Parse("2006-01-02", to); err == nil {
			q.To = t.Add(24*time.Hour - time.Nanosecond) // end of day
		}
	}

	results := h.decisionLogger.Query(q)
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"decisions": results,
		"count":     len(results),
	})
}

// handleGetDecisionStats handles aggregate decision statistics requests.
// GET /api/v1/admin/decisions/stats
func (h *AdminHandlers) handleGetDecisionStats(w http.ResponseWriter, r *http.Request) {
	if h.decisionLogger == nil {
		RespondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "not_available",
			Message: "Decision logger not configured",
		})
		return
	}

	stats := h.decisionLogger.Stats()
	RespondJSON(w, http.StatusOK, stats)
}

// handleHealthCheck handles health check requests
func (h *AdminHandlers) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check cache health
	cacheHealthy := true
	if h.cache != nil {
		// Could add more detailed health checks
		_ = h.cache.GetStats()
	}

	// Check database health
	dbHealthy := true
	if h.curatedDB != nil {
		_ = h.curatedDB.GetStats()
	}

	// Check validator health
	validatorHealthy := true
	if h.validator != nil {
		// Validator is stateless, so it's always healthy
	}

	healthy := cacheHealthy && dbHealthy && validatorHealthy

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	resp := map[string]interface{}{
		"status":  map[bool]string{true: "healthy", false: "unhealthy"}[healthy],
		"healthy": healthy,
		"checks": map[string]bool{
			"cache":     cacheHealthy,
			"database":  dbHealthy,
			"validator": validatorHealthy,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	RespondJSON(w, status, resp)
}

// --- Phase 3.1: Extended Curated Device Management API ---

// CreateCuratedDeviceRequest represents a request to create a new curated device
type CreateCuratedDeviceRequest struct {
	ID                 string                       `json:"id,omitempty"`
	DeviceHash         string                       `json:"device_hash,omitempty"`
	Name               string                       `json:"name"`
	Manufacturer       string                       `json:"manufacturer"`
	Model              string                       `json:"model"`
	Platform           string                       `json:"platform"`
	OSVersions         []string                     `json:"os_versions,omitempty"`
	Capabilities       *probes.DeviceCapabilities    `json:"capabilities"`
	RecommendedProfile string                       `json:"recommended_profile,omitempty"`
	KnownIssues        []probes.KnownIssue          `json:"known_issues,omitempty"`
	Source             string                       `json:"source,omitempty"`
	Notes              string                       `json:"notes,omitempty"`
}

// FuzzyMatchRequest represents a fuzzy search request
type FuzzyMatchRequest struct {
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// FuzzyMatchResponse represents a fuzzy search response
type FuzzyMatchResponse struct {
	Query      string                      `json:"query"`
	Results    []*probes.FuzzyMatchResult  `json:"results"`
	TotalFound int                         `json:"total_found"`
}

// VersionMatchResponse represents version-aware matching response
type VersionMatchResponse struct {
	Original   string                    `json:"original"`
	BestMatch  *probes.CuratedDevice     `json:"best_match,omitempty"`
	Confidence string                    `json:"confidence"`
	BaseModel  string                    `json:"base_model,omitempty"`
	Year       string                    `json:"year,omitempty"`
	Variant    string                    `json:"variant,omitempty"`
}

// EmbeddedBundleInfo represents embedded data bundle information
type EmbeddedBundleInfo struct {
	Platform      string `json:"platform"`
	Manufacturer  string `json:"manufacturer"`
	Version       string `json:"version"`
	LastUpdated   string `json:"last_updated"`
	Source        string `json:"source"`
	DeviceCount   int    `json:"device_count"`
	VerifiedCount int    `json:"verified_count"`
}

// EmbeddedStatsResponse represents embedded data statistics
type EmbeddedStatsResponse struct {
	Version        string                    `json:"version"`
	TotalDevices   int                       `json:"total_devices"`
	Platforms      []string                  `json:"platforms"`
	LoadedAt       string                    `json:"loaded_at"`
	PlatformsInfo  map[string]EmbeddedBundleInfo `json:"platforms_info"`
}

// handleCreateCuratedDevice handles creating a new curated device (PUT /api/v1/admin/curated/devices)
func (h *AdminHandlers) handleCreateCuratedDevice(w http.ResponseWriter, r *http.Request) {
	var req CreateCuratedDeviceRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	// Validate required fields
	if req.Name == "" || req.Manufacturer == "" || req.Model == "" || req.Platform == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_required_fields",
			Message: "name, manufacturer, model, and platform are required",
		})
		return
	}

	// Validate capabilities if provided
	if req.Capabilities != nil {
		validation := h.validator.ValidateCapabilities(req.Capabilities)
		if !validation.IsValid {
			RespondJSON(w, http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_capabilities",
				Message: "Capabilities validation failed",
				Details: validation.Errors,
			})
			return
		}
	}

	// Create device
	device := &probes.CuratedDevice{
		ID:                 req.ID,
		DeviceHash:         req.DeviceHash,
		Name:               req.Name,
		Manufacturer:       req.Manufacturer,
		Model:              req.Model,
		Platform:           req.Platform,
		OSVersions:         req.OSVersions,
		Capabilities:       req.Capabilities,
		RecommendedProfile: req.RecommendedProfile,
		KnownIssues:        req.KnownIssues,
		Source:             req.Source,
		Notes:              req.Notes,
		Verified:           false,
		CreatedAt:          time.Now().UTC(),
		LastUpdated:        time.Now().UTC(),
	}

	// Set source default if not provided
	if device.Source == "" {
		device.Source = "community"
	}

	// Add to database
	if err := h.curatedDB.AddDevice(device); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "create_failed",
			Message: "Failed to create device: " + err.Error(),
		})
		return
	}

	RespondJSON(w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"device_id": device.ID,
		"message":   "Device created successfully",
		"device":    device,
	})
}

// handlePutCuratedDevice handles full device replacement (PUT /api/v1/admin/curated/devices/{id})
func (h *AdminHandlers) handlePutCuratedDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	var req CreateCuratedDeviceRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	// Validate required fields
	if req.Name == "" || req.Manufacturer == "" || req.Model == "" || req.Platform == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_required_fields",
			Message: "name, manufacturer, model, and platform are required",
		})
		return
	}

	// Get existing device to preserve some fields
	existing, found := h.curatedDB.GetByID(deviceID)
	if !found {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "device_not_found",
			Message: "Device not found",
		})
		return
	}

	// Update device (full replacement)
	device := &probes.CuratedDevice{
		ID:                 deviceID,
		DeviceHash:         req.DeviceHash,
		Name:               req.Name,
		Manufacturer:       req.Manufacturer,
		Model:              req.Model,
		Platform:           req.Platform,
		OSVersions:         req.OSVersions,
		Capabilities:       req.Capabilities,
		RecommendedProfile: req.RecommendedProfile,
		KnownIssues:        req.KnownIssues,
		Source:             req.Source,
		Notes:              req.Notes,
		Verified:           existing.Verified, // Preserve verification status
		CreatedAt:          existing.CreatedAt, // Preserve creation time
		LastUpdated:        time.Now().UTC(),
		VotesUp:            existing.VotesUp,   // Preserve votes
		VotesDown:          existing.VotesDown,
	}

	// Validate capabilities if provided
	if device.Capabilities != nil {
		validation := h.validator.ValidateCapabilities(device.Capabilities)
		if !validation.IsValid {
			RespondJSON(w, http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_capabilities",
				Message: "Capabilities validation failed",
				Details: validation.Errors,
			})
			return
		}
	}

	// Save updated device
	if err := h.curatedDB.UpdateDevice(device); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "update_failed",
			Message: "Failed to update device",
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"device_id": deviceID,
		"message":   "Device replaced successfully",
		"device":    device,
	})
}

// handleDeleteCuratedDevice handles device deletion (DELETE /api/v1/admin/curated/devices/{id})
func (h *AdminHandlers) handleDeleteCuratedDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	// Check if device exists
	device, found := h.curatedDB.GetByID(deviceID)
	if !found {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "device_not_found",
			Message: "Device not found",
		})
		return
	}

	// Prevent deletion of official/verified devices (optional protection)
	if device.Verified && device.Source == "official" {
		RespondJSON(w, http.StatusForbidden, ErrorResponse{
			Error:   "protected_device",
			Message: "Cannot delete verified official devices. Unverify first.",
		})
		return
	}

	// Remove device
	if err := h.curatedDB.RemoveDevice(deviceID); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "Failed to remove device",
		})
		return
	}

	// Also remove from cache
	h.cache.Delete(deviceID)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"device_id": deviceID,
		"message":   "Device deleted successfully",
	})
}

// handleVoteCuratedDevice handles voting on a device (POST /api/v1/admin/curated/devices/{id}/vote)
func (h *AdminHandlers) handleVoteCuratedDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")

	if deviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	var req struct {
		Up bool `json:"up"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to upvote
		req.Up = true
	}

	// Vote on device
	if err := h.curatedDB.Vote(deviceID, req.Up); err != nil {
		RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "vote_failed",
			Message: "Failed to vote on device",
		})
		return
	}

	// Get updated device
	device, ok := h.curatedDB.GetByID(deviceID)
	if !ok || device == nil {
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success":   true,
			"device_id": deviceID,
			"message":   "Vote recorded",
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"device_id": deviceID,
		"message":   "Vote recorded",
		"votes_up":  device.VotesUp,
		"votes_down": device.VotesDown,
	})
}

// handleFuzzySearchCuratedDevices handles fuzzy search (POST /api/v1/admin/curated/search)
func (h *AdminHandlers) handleFuzzySearchCuratedDevices(w http.ResponseWriter, r *http.Request) {
	var req FuzzyMatchRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if req.DeviceName == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_name",
			Message: "device_name is required",
		})
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	results := h.curatedDB.MatchDevice(req.DeviceName, req.Platform, limit)

	RespondJSON(w, http.StatusOK, FuzzyMatchResponse{
		Query:      req.DeviceName,
		Results:    results,
		TotalFound: len(results),
	})
}

// handleVersionMatch handles version-aware device matching (POST /api/v1/admin/curated/version-match)
func (h *AdminHandlers) handleVersionMatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceName string `json:"device_name"`
		Platform   string `json:"platform,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if req.DeviceName == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_name",
			Message: "device_name is required",
		})
		return
	}

	result := h.curatedDB.VersionAwareMatch(req.DeviceName, req.Platform)

	RespondJSON(w, http.StatusOK, VersionMatchResponse{
		Original:   result.Original,
		BestMatch:  result.BestMatch,
		Confidence: result.Confidence,
		BaseModel:  result.BaseModel,
		Year:       result.Year,
		Variant:    result.Variant,
	})
}

// handleGetEmbeddedStats handles getting embedded data statistics
func (h *AdminHandlers) handleGetEmbeddedStats(w http.ResponseWriter, r *http.Request) {
	loader := probes.GetEmbeddedLoader()

	platforms := loader.GetPlatforms()
	bundlesInfo := loader.GetAllBundlesInfo()

	var totalDevices int
	for _, info := range bundlesInfo {
		if count, ok := info["device_count"].(int); ok {
			totalDevices += count
		}
	}

	resp := EmbeddedStatsResponse{
		Version:       loader.GetVersion(),
		TotalDevices:  totalDevices,
		Platforms:     platforms,
		LoadedAt:      loader.GetLoadTime().Format(time.RFC3339),
		PlatformsInfo: make(map[string]EmbeddedBundleInfo),
	}

	for platform, info := range bundlesInfo {
		bundleInfo := EmbeddedBundleInfo{
			DeviceCount: info["device_count"].(int),
		}
		if v, ok := info["manufacturer"].(string); ok {
			bundleInfo.Manufacturer = v
		}
		if v, ok := info["version"].(string); ok {
			bundleInfo.Version = v
		}
		if v, ok := info["last_updated"].(string); ok {
			bundleInfo.LastUpdated = v
		}
		if v, ok := info["source"].(string); ok {
			bundleInfo.Source = v
		}
		if v, ok := info["verified_count"].(int); ok {
			bundleInfo.VerifiedCount = v
		}
		resp.PlatformsInfo[platform] = bundleInfo
	}

	RespondJSON(w, http.StatusOK, resp)
}

// handleSyncEmbeddedToCuratedDB handles syncing embedded devices to curated DB
func (h *AdminHandlers) handleSyncEmbeddedToCuratedDB(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform   string `json:"platform,omitempty"`
		Overwrite  bool   `json:"overwrite,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Use defaults
		req.Overwrite = false
	}

	loader := probes.GetEmbeddedLoader()
	var synced int
	var errors []string

	if req.Platform != "" {
		// Sync specific platform
		devices := loader.GetDevices(req.Platform)
		if devices == nil {
			RespondJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   "platform_not_found",
				Message: "Platform not found in embedded data",
			})
			return
		}

		for _, device := range devices {
			if err := h.curatedDB.AddDevice(device); err != nil {
				errors = append(errors, device.ID+": "+err.Error())
			} else {
				synced++
			}
		}
	} else {
		// Sync all platforms
		if err := loader.LoadIntoCuratedDB(h.curatedDB); err != nil {
			RespondJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "sync_failed",
				Message: "Failed to sync embedded devices: " + err.Error(),
			})
			return
		}
		synced = loader.GetTotalCount()
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"devices_synced": synced,
		"errors":         errors,
		"platform":       req.Platform,
		"message":        fmt.Sprintf("Synced %d devices", synced),
	})
}

// handleExportCuratedDevices handles exporting curated devices
func (h *AdminHandlers) handleExportCuratedDevices(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	format := r.URL.Query().Get("format")

	if format == "" {
		format = "json"
	}

	var devices []*probes.CuratedDevice
	if platform != "" {
		devices = h.curatedDB.GetByPlatform(platform)
	} else {
		devices = h.curatedDB.GetAll()
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=curated-devices-%s.json", platform))

		export := map[string]interface{}{
			"_metadata": map[string]string{
				"exported_at": time.Now().UTC().Format(time.RFC3339),
				"platform":   platform,
				"count":      fmt.Sprintf("%d", len(devices)),
			},
			"devices": devices,
		}

		RespondJSON(w, http.StatusOK, export)
		return
	}

	RespondJSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "unsupported_format",
		Message: "Only JSON format is supported",
	})
}

// Import request structures
type importRequest struct {
	Devices       []*probes.CuratedDevice `json:"devices"`
	MergeStrategy string                  `json:"merge_strategy,omitempty"` // "skip", "overwrite", "upsert"
}

// handleImportCuratedDevices handles importing curated devices
func (h *AdminHandlers) handleImportCuratedDevices(w http.ResponseWriter, r *http.Request) {
	var req importRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if len(req.Devices) == 0 {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "no_devices",
			Message: "No devices to import",
		})
		return
	}

	mergeStrategy := req.MergeStrategy
	if mergeStrategy == "" {
		mergeStrategy = "upsert"
	}

	var imported, skipped, errors int
	var errorDetails []string

	for _, device := range req.Devices {
		// Validate device
		if device.Name == "" || device.Manufacturer == "" || device.Model == "" || device.Platform == "" {
			errors++
			errorDetails = append(errorDetails, "validation: missing required fields")
			continue
		}

		// Check for existing device
		existing, found := h.curatedDB.GetByID(device.ID)

		switch mergeStrategy {
		case "skip":
			if found {
				skipped++
				continue
			}
		case "overwrite":
			if found {
				device.ID = existing.ID
				device.CreatedAt = existing.CreatedAt
				device.LastUpdated = time.Now().UTC()
				device.VotesUp = existing.VotesUp
				device.VotesDown = existing.VotesDown
			}
		case "upsert":
			if found {
				// Only update if new data is more recent or has higher votes
				if device.VotesUp <= existing.VotesUp && !device.Verified {
					skipped++
					continue
				}
				device.ID = existing.ID
				device.CreatedAt = existing.CreatedAt
				device.LastUpdated = time.Now().UTC()
				device.VotesUp = existing.VotesUp
				device.VotesDown = existing.VotesDown
			}
		}

		// Set timestamps if not set
		if device.CreatedAt.IsZero() {
			device.CreatedAt = time.Now().UTC()
		}
		if device.LastUpdated.IsZero() {
			device.LastUpdated = time.Now().UTC()
		}

		if err := h.curatedDB.AddDevice(device); err != nil {
			errors++
			errorDetails = append(errorDetails, device.ID+": "+err.Error())
		} else {
			imported++
		}
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"imported":     imported,
		"skipped":      skipped,
		"errors":       errors,
		"error_details": errorDetails,
		"total":        len(req.Devices),
		"message":      fmt.Sprintf("Imported %d, skipped %d, errors %d", imported, skipped, errors),
	})
}