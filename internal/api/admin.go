// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/tenkile/tenkile/internal/probes"
)

// AdminHandlers holds admin-related API handlers
type AdminHandlers struct {
	validator   *probes.Validator
	cache       *probes.CapabilityCache
	curatedDB   *probes.CuratedDatabase
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