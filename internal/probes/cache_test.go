// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"os"
	"testing"
	"time"
)

// TestCapabilityCache_New tests creating a new cache
func TestCapabilityCache_New(t *testing.T) {
	// Test with nil config (should use defaults)
	cache, err := NewCapabilityCache(nil)
	if err != nil {
		t.Fatalf("Expected no error with nil config, got: %v", err)
	}
	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
	cache.Close()

	// Test with custom config
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	config := &CacheConfig{
		MemoryTTL:      time.Hour,
		SQLiteTTL:      time.Hour * 24,
		EnableSQLite:   true,
		MaxMemoryItems: 100,
		DatabasePath:   tmpDB,
	}

	cache, err = NewCapabilityCache(config)
	if err != nil {
		t.Fatalf("Expected no error with valid config, got: %v", err)
	}
	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
	cache.Close()
}

// TestCapabilityCache_SetAndGet tests basic cache operations
func TestCapabilityCache_SetAndGet(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		EnableSQLite:   true,
		DatabasePath:   tmpDB,
		MemoryTTL:      time.Hour,
		MaxMemoryItems: 1000,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Create test capabilities
	caps := &DeviceCapabilities{
		DeviceID:      "test-device-001",
		MaxWidth:      1920,
		MaxHeight:     1080,
		VideoCodecs:   []string{"h264", "hevc"},
		AudioCodecs:   []string{"aac", "mp3"},
		SupportsHDR:   true,
		MaxBitrate:    50000000,
	}

	// Test Set
	err = cache.Set("test-device-001", caps, "probe")
	if err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}

	// Test Get
	retrieved, found := cache.Get("test-device-001")
	if !found {
		t.Fatal("Expected to find cached capabilities")
	}
	if retrieved == nil {
		t.Fatal("Expected non-nil capabilities")
	}
	if retrieved.DeviceID != "test-device-001" {
		t.Errorf("Expected device_id 'test-device-001', got '%s'", retrieved.DeviceID)
	}
	if retrieved.MaxWidth != 1920 {
		t.Errorf("Expected MaxWidth 1920, got %d", retrieved.MaxWidth)
	}
}

// TestCapabilityCache_Get_Miss tests cache miss
func TestCapabilityCache_Get_Miss(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		EnableSQLite:   true,
		DatabasePath:   tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	_, found := cache.Get("non-existent-device")
	if found {
		t.Error("Expected cache miss for non-existent device")
	}
}

// TestCapabilityCache_Delete tests cache deletion
func TestCapabilityCache_Delete(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		EnableSQLite:   true,
		DatabasePath:   tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	caps := &DeviceCapabilities{
		DeviceID: "test-device-001",
	}

	// Set and verify
	cache.Set("test-device-001", caps, "probe")
	_, found := cache.Get("test-device-001")
	if !found {
		t.Fatal("Expected to find cached capabilities before delete")
	}

	// Delete
	cache.Delete("test-device-001")

	// Verify deleted
	_, found = cache.Get("test-device-001")
	if found {
		t.Error("Expected cache miss after delete")
	}
}

// TestCapabilityCache_Clear tests clearing the cache
func TestCapabilityCache_Clear(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		EnableSQLite:   true,
		DatabasePath:   tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Add items
	caps1 := &DeviceCapabilities{DeviceID: "device-1"}
	caps2 := &DeviceCapabilities{DeviceID: "device-2"}
	cache.Set("device-1", caps1, "probe")
	cache.Set("device-2", caps2, "probe")

	// Verify items exist
	_, found := cache.Get("device-1")
	if !found {
		t.Error("Expected device-1 to exist")
	}
	_, found = cache.Get("device-2")
	if !found {
		t.Error("Expected device-2 to exist")
	}

	// Clear
	cache.Clear()

	// Verify stats are reset (check before any new Gets that would increment counters)
	stats := cache.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Expected stats to be reset, got hits=%d, misses=%d", stats.Hits, stats.Misses)
	}

	// Verify items are gone
	_, found = cache.Get("device-1")
	if found {
		t.Error("Expected device-1 to be cleared")
	}
	_, found = cache.Get("device-2")
	if found {
		t.Error("Expected device-2 to be cleared")
	}
}

// TestCapabilityCache_GetStats tests cache statistics
func TestCapabilityCache_GetStats(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		EnableSQLite:   true,
		DatabasePath:   tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Initial stats
	stats := cache.GetStats()
	if stats.MemorySize != 0 {
		t.Errorf("Expected initial memory size 0, got %d", stats.MemorySize)
	}

	// Add item and trigger miss
	caps := &DeviceCapabilities{DeviceID: "stats-test"}
	cache.Set("stats-test", caps, "probe")

	// Get non-existent to trigger miss
	cache.Get("non-existent")
	stats = cache.GetStats()
	if stats.Misses < 1 {
		t.Errorf("Expected at least 1 miss, got %d", stats.Misses)
	}
}

// TestCapabilityCache_InvalidInput tests handling of invalid input
func TestCapabilityCache_InvalidInput(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		DatabasePath: tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Test Set with empty device ID
	err = cache.Set("", &DeviceCapabilities{}, "probe")
	if err == nil {
		t.Error("Expected error for empty device ID")
	}

	// Test Set with nil capabilities
	err = cache.Set("device-1", nil, "probe")
	if err == nil {
		t.Error("Expected error for nil capabilities")
	}

	// Test Get with empty device ID
	_, found := cache.Get("")
	if found {
		t.Error("Expected cache miss for empty device ID")
	}
}

// TestCapabilityCache_BatchOperations tests batch operations
func TestCapabilityCache_BatchOperations(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		DatabasePath: tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Test BatchSet
	entries := map[string]*DeviceCapabilities{
		"batch-1": {DeviceID: "batch-1"},
		"batch-2": {DeviceID: "batch-2"},
		"batch-3": {DeviceID: "batch-3"},
	}

	err = cache.BatchSet(entries, "probe")
	if err != nil {
		t.Fatalf("BatchSet() returned error: %v", err)
	}

	// Test BatchGet
	deviceIDs := []string{"batch-1", "batch-2", "batch-3", "non-existent"}
	results, err := cache.BatchGet(deviceIDs)
	if err != nil {
		t.Fatalf("BatchGet() returned error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	if _, ok := results["batch-1"]; !ok {
		t.Error("Expected batch-1 in results")
	}
	if _, ok := results["non-existent"]; ok {
		t.Error("Did not expect non-existent in results")
	}
}

// TestCapabilityCache_MemoryOnly tests memory-only cache (SQLite disabled)
func TestCapabilityCache_MemoryOnly(t *testing.T) {
	cache, err := NewCapabilityCache(&CacheConfig{
		EnableSQLite:   false,
		MemoryTTL:      time.Hour,
		MaxMemoryItems: 100,
	})
	if err != nil {
		t.Fatalf("Failed to create memory-only cache: %v", err)
	}
	defer cache.Close()

	caps := &DeviceCapabilities{DeviceID: "mem-only"}
	err = cache.Set("mem-only", caps, "probe")
	if err != nil {
		t.Fatalf("Set() returned error: %v", err)
	}

	retrieved, found := cache.Get("mem-only")
	if !found {
		t.Error("Expected to find cached capabilities")
	}
	if retrieved.DeviceID != "mem-only" {
		t.Errorf("Expected device_id 'mem-only', got '%s'", retrieved.DeviceID)
	}
}

// TestCapabilityCache_Close tests cache cleanup
func TestCapabilityCache_Close(t *testing.T) {
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	cache, err := NewCapabilityCache(&CacheConfig{
		DatabasePath: tmpDB,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	err = cache.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify we can close again without error
	err = cache.Close()
	if err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

// createTempDB creates a temporary database file for testing
func createTempDB(t *testing.T) string {
	tmpFile, err := os.CreateTemp("", "cache-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}
