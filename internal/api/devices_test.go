// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tenkile/tenkile/internal/probes"
)

// createTestDeviceHandlers creates a DeviceHandlers with in-memory dependencies
func createTestDeviceHandlers(t *testing.T) *DeviceHandlers {
	t.Helper()
	validator := probes.NewValidator()
	cache, err := probes.NewCapabilityCache(&probes.CacheConfig{
		EnableSQLite:   false,
		MaxMemoryItems: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	curatedDB := probes.NewCuratedDatabase()
	return NewDeviceHandlers(validator, cache, curatedDB)
}

func TestDeviceHandlers_NewDeviceHandlers(t *testing.T) {
	validator := probes.NewValidator()
	cache, _ := probes.NewCapabilityCache(nil)
	curatedDB := probes.NewCuratedDatabase()

	handlers := NewDeviceHandlers(validator, cache, curatedDB)

	if handlers == nil {
		t.Fatal("Expected non-nil handlers")
	}
	if handlers.validator == nil {
		t.Error("Expected validator to be set")
	}
	if handlers.cache == nil {
		t.Error("Expected cache to be set")
	}
	if handlers.curatedDB == nil {
		t.Error("Expected curatedDB to be set")
	}
}

func TestDeviceHandlers_handleProbeReport_Success(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	caps := &probes.DeviceCapabilities{
		DeviceID:         "test-device-001",
		Platform:         "web",
		VideoCodecs:      []string{"h264", "hevc"},
		AudioCodecs:      []string{"aac"},
		ContainerFormats: []string{"mp4", "mkv"},
		MaxWidth:         1920,
		MaxHeight:        1080,
	}

	reqBody := ProbeReportRequest{
		Capabilities: caps,
		Source:       "probe",
		Version:      "1.0.0",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/probe/report", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.handleProbeReport(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result ProbeReportResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if !result.Success {
		t.Error("Expected success to be true")
	}
	if result.DeviceID != "test-device-001" {
		t.Errorf("Expected device_id 'test-device-001', got '%s'", result.DeviceID)
	}
}

func TestDeviceHandlers_handleProbeReport_MissingCapabilities(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	reqBody := ProbeReportRequest{Capabilities: nil}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/probe/report", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.handleProbeReport(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleProbeReport_InvalidJSON(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/probe/report", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handlers.handleProbeReport(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleGetCapabilities_Success(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	caps := &probes.DeviceCapabilities{
		DeviceID:    "lookup-device",
		VideoCodecs: []string{"h264", "vp9"},
		AudioCodecs: []string{"aac", "opus"},
	}
	handlers.cache.Set("lookup-device", caps, "probe")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?device_id=lookup-device", nil)
	w := httptest.NewRecorder()

	handlers.handleGetCapabilities(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result CapabilitiesResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if !result.Found {
		t.Error("Expected found to be true")
	}
}

func TestDeviceHandlers_handleGetCapabilities_NotFound(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?device_id=non-existent", nil)
	w := httptest.NewRecorder()

	handlers.handleGetCapabilities(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleGetCapabilities_MissingID(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	w := httptest.NewRecorder()

	handlers.handleGetCapabilities(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleValidateCapabilities_Success(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	caps := &probes.DeviceCapabilities{
		DeviceID:    "validate-device",
		VideoCodecs: []string{"h264", "vp9"},
		AudioCodecs: []string{"aac", "opus"},
		MaxWidth:    1920,
		MaxHeight:   1080,
	}

	reqBody := map[string]*probes.DeviceCapabilities{
		"capabilities": caps,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/capabilities", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.handleValidateCapabilities(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleValidateCapabilities_MissingCapabilities(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capabilities", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.handleValidateCapabilities(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleGetCuratedDevices(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/curated", nil)
	w := httptest.NewRecorder()

	handlers.handleGetCuratedDevices(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDeviceHandlers_handleSearchDevices(t *testing.T) {
	handlers := createTestDeviceHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/search?platform=ios", nil)
	w := httptest.NewRecorder()

	handlers.handleSearchDevices(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
