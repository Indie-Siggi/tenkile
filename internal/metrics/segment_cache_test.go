// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package metrics

import (
	"context"
	"io"
	"os"
	"testing"
	"time"
)

func TestSegmentCacheCreation(t *testing.T) {
	config := SegmentCacheConfig{
		MemoryCacheSize: 10 << 20, // 10MB
		MemoryMaxItems:  100,
		DiskCacheDir:    "",
		DiskCacheSize:   1 << 30, // 1GB
		DefaultTTL:      time.Hour,
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Error("expected non-nil cache")
	}

	stats := cache.Stats()
	if stats.L1Max != 10<<20 {
		t.Errorf("expected L1Max to be 10MB, got %d", stats.L1Max)
	}
	if stats.L2Max != 1<<30 {
		t.Errorf("expected L2Max to be 1GB, got %d", stats.L2Max)
	}
}

func TestSegmentCacheDefaults(t *testing.T) {
	// Test with zero values - should use defaults
	config := SegmentCacheConfig{}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	stats := cache.Stats()
	// Default memory cache is 100MB
	if stats.L1Max != 100<<20 {
		t.Errorf("expected default L1Max to be 100MB, got %d", stats.L1Max)
	}
	// Default disk cache is 10GB
	if stats.L2Max != 10<<30 {
		t.Errorf("expected default L2Max to be 10GB, got %d", stats.L2Max)
	}
}

func TestSegmentCacheGetSet(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Initial get should miss
	data, ok := cache.Get(ctx, "media1", "1080p", "seg1")
	if ok || data != nil {
		t.Error("expected initial cache miss")
	}

	// Set data
	testData := []byte("test segment data")
	err = cache.Set(ctx, "media1", "1080p", "seg1", testData)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Should now hit
	data, ok = cache.Get(ctx, "media1", "1080p", "seg1")
	if !ok {
		t.Error("expected cache hit after set")
	}
	if string(data) != "test segment data" {
		t.Errorf("expected data to match, got '%s'", string(data))
	}
}

func TestSegmentCacheL1Only(t *testing.T) {
	config := SegmentCacheConfig{
		MemoryCacheSize: 1 << 20, // 1MB - very small
		MemoryMaxItems:  10,
		DiskCacheDir:    "", // No disk cache
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Fill cache with small segments
	for i := 0; i < 5; i++ {
		data := []byte("segment data")
		err := cache.Set(ctx, "media1", "1080p", string(rune('0'+i)), data)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}
	}

	// Should have entries
	stats := cache.Stats()
	if stats.L1Count == 0 {
		t.Error("expected L1 to have entries")
	}
}

func TestSegmentCacheEviction(t *testing.T) {
	config := SegmentCacheConfig{
		MemoryCacheSize: 100, // 100 bytes - very small
		MemoryMaxItems:  3,
		DiskCacheDir:    "",
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Fill cache beyond limits
	for i := 0; i < 5; i++ {
		data := []byte("xxxxxxxxxxxxxxxxxxxx") // 20 bytes each
		err := cache.Set(ctx, "media1", "1080p", string(rune('a'+i)), data)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}
	}

	// Cache should evict old entries
	stats := cache.Stats()
	if stats.L1Count > 3 {
		t.Errorf("expected L1 count <= 3 (max items), got %d", stats.L1Count)
	}
}

func TestSegmentCacheInvalidate(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set some data
	err = cache.Set(ctx, "media1", "1080p", "seg1", []byte("data1"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}
	err = cache.Set(ctx, "media1", "1080p", "seg2", []byte("data2"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Verify both exist
	_, ok1 := cache.Get(ctx, "media1", "1080p", "seg1")
	_, ok2 := cache.Get(ctx, "media1", "1080p", "seg2")
	if !ok1 || !ok2 {
		t.Error("expected both segments to exist")
	}

	// Invalidate one
	cache.Invalidate("media1", "1080p", "seg1")

	// seg1 should be gone
	_, ok1 = cache.Get(ctx, "media1", "1080p", "seg1")
	// seg2 should still exist
	_, ok2 = cache.Get(ctx, "media1", "1080p", "seg2")

	if ok1 {
		t.Error("expected seg1 to be invalidated")
	}
	if !ok2 {
		t.Error("expected seg2 to still exist")
	}
}

func TestSegmentCacheInvalidateMedia(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set data for different media
	err = cache.Set(ctx, "media1", "1080p", "seg1", []byte("data1"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}
	err = cache.Set(ctx, "media1", "720p", "seg1", []byte("data2"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}
	err = cache.Set(ctx, "media2", "1080p", "seg1", []byte("data3"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Invalidate all of media1
	cache.InvalidateMedia("media1")

	// media1 should be gone
	_, ok1 := cache.Get(ctx, "media1", "1080p", "seg1")
	_, ok2 := cache.Get(ctx, "media1", "720p", "seg1")
	// media2 should still exist
	_, ok3 := cache.Get(ctx, "media2", "1080p", "seg1")

	if ok1 {
		t.Error("expected media1:1080p:seg1 to be invalidated")
	}
	if ok2 {
		t.Error("expected media1:720p:seg1 to be invalidated")
	}
	if !ok3 {
		t.Error("expected media2:1080p:seg1 to still exist")
	}
}

func TestSegmentCacheStats(t *testing.T) {
	config := SegmentCacheConfig{
		MemoryCacheSize: 1000,
		DiskCacheSize:   10000,
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	stats := cache.Stats()

	if stats.L1Max != 1000 {
		t.Errorf("expected L1Max to be 1000, got %d", stats.L1Max)
	}
	if stats.L2Max != 10000 {
		t.Errorf("expected L2Max to be 10000, got %d", stats.L2Max)
	}
	if stats.L1Count != 0 {
		t.Errorf("expected L1Count to be 0, got %d", stats.L1Count)
	}
}

func TestSegmentCacheDiskCache(t *testing.T) {
	// Create temp directory for disk cache
	tempDir, err := os.MkdirTemp("", "tenkile-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := SegmentCacheConfig{
		MemoryCacheSize: 1 << 20,
		DiskCacheDir:    tempDir,
		DiskCacheSize:   10 << 20,
		DefaultTTL:      time.Hour,
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set data
	testData := []byte("large segment data for disk cache")
	err = cache.Set(ctx, "media1", "1080p", "seg1", testData)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Verify disk cache file exists
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	
	if len(files) == 0 {
		t.Error("expected disk cache files to exist")
	}
}

func TestSegmentCacheDiskCacheWithEviction(t *testing.T) {
	// Create temp directory for disk cache
	tempDir, err := os.MkdirTemp("", "tenkile-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := SegmentCacheConfig{
		MemoryCacheSize: 100, // Very small memory cache
		DiskCacheDir:    tempDir,
		DiskCacheSize:   1000, // Small disk cache
		DefaultTTL:      time.Hour,
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Fill cache
	for i := 0; i < 20; i++ {
		data := make([]byte, 100) // 100 bytes each
		err := cache.Set(ctx, "media1", "1080p", string(rune('0'+i)), data)
		if err != nil {
			t.Fatalf("failed to set cache: %v", err)
		}
	}

	// Should have disk cache entries
	files, _ := os.ReadDir(tempDir)
	t.Logf("disk cache has %d files", len(files))
}

func TestSegmentCacheOnEvictCallback(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{
		MemoryCacheSize: 50,
		MemoryMaxItems:  2,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	evicted := make(map[string]bool)
	cache.SetOnEvict(func(key string, data []byte) {
		evicted[key] = true
	})

	ctx := context.Background()

	// Fill cache to trigger eviction
	for i := 0; i < 5; i++ {
		data := []byte("xxxxxxxxxxxx") // 12 bytes
		cache.Set(ctx, "media1", "1080p", string(rune('a'+i)), data)
	}

	// Some keys should have been evicted
	if len(evicted) == 0 {
		t.Error("expected some entries to be evicted")
	}
}

func TestSegmentCacheClose(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	ctx := context.Background()
	cache.Set(ctx, "media1", "1080p", "seg1", []byte("data"))

	err = cache.Close()
	if err != nil {
		t.Fatalf("failed to close cache: %v", err)
	}

	// Cache should be cleared after close
	stats := cache.Stats()
	if stats.L1Count != 0 {
		t.Errorf("expected L1Count to be 0 after close, got %d", stats.L1Count)
	}
}

func TestSegmentCacheStreamToCache(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	
	// Create a simple reader
	reader := &testReader{data: []byte("test data from reader")}
	
	err = cache.StreamToCache(ctx, "media1", "1080p", "seg1", reader)
	if err != nil {
		t.Fatalf("failed to stream to cache: %v", err)
	}

	// Verify data was cached
	data, ok := cache.Get(ctx, "media1", "1080p", "seg1")
	if !ok {
		t.Error("expected cache hit after streaming")
	}
	if string(data) != "test data from reader" {
		t.Errorf("expected data to match, got '%s'", string(data))
	}
}

// testReader implements io.Reader for testing
type testReader struct {
	data    []byte
	pos     int
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestSegmentCacheKeyUniqueness(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Different keys should be stored separately
	cache.Set(ctx, "media1", "1080p", "seg1", []byte("data1"))
	cache.Set(ctx, "media1", "720p", "seg1", []byte("data2"))
	cache.Set(ctx, "media2", "1080p", "seg1", []byte("data3"))

	// All should exist
	_, ok1 := cache.Get(ctx, "media1", "1080p", "seg1")
	_, ok2 := cache.Get(ctx, "media1", "720p", "seg1")
	_, ok3 := cache.Get(ctx, "media2", "1080p", "seg1")

	if !ok1 {
		t.Error("expected media1:1080p:seg1")
	}
	if !ok2 {
		t.Error("expected media1:720p:seg1")
	}
	if !ok3 {
		t.Error("expected media2:1080p:seg1")
	}
}

func TestSegmentCacheDiskCachePromotion(t *testing.T) {
	// Create temp directory for disk cache
	tempDir, err := os.MkdirTemp("", "tenkile-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := SegmentCacheConfig{
		MemoryCacheSize: 10, // Very small - will evict L1 immediately
		DiskCacheDir:    tempDir,
		DiskCacheSize:   10 << 20,
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set data - will go to disk since L1 is too small
	err = cache.Set(ctx, "media1", "1080p", "seg1", []byte("test data"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Clear L1 (by filling with other data)
	for i := 0; i < 10; i++ {
		cache.Set(ctx, "mediaX", "1080p", string(rune('0'+i)), []byte("x"))
	}

	// Get should still work (will be promoted from L2 to L1)
	data, ok := cache.Get(ctx, "media1", "1080p", "seg1")
	if !ok {
		t.Error("expected cache hit from disk cache")
	}
	if string(data) != "test data" {
		t.Errorf("expected data to match, got '%s'", string(data))
	}
}

func TestSegmentCacheMakeKey(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	key1 := cache.makeKey("media1", "1080p", "seg1")
	key2 := cache.makeKey("media1", "1080p", "seg1")
	key3 := cache.makeKey("media1", "720p", "seg1")

	// Same inputs should produce same key
	if key1 != key2 {
		t.Errorf("expected same key for same inputs, got %s and %s", key1, key2)
	}

	// Different inputs should produce different key
	if key1 == key3 {
		t.Errorf("expected different key for different inputs")
	}

	// Key should start with media ID
	if len(key1) < 7 || key1[:7] != "media1:" {
		t.Errorf("expected key to start with media1:, got %s", key1)
	}
}

func TestSegmentCacheEmptyDiskDir(t *testing.T) {
	config := SegmentCacheConfig{
		MemoryCacheSize: 1 << 20,
		DiskCacheDir:    "", // No disk cache
		DiskCacheSize:   10 << 20,
	}

	cache, err := NewSegmentCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	
	// Setting should work without disk cache
	err = cache.Set(ctx, "media1", "1080p", "seg1", []byte("test data"))
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Getting should work
	data, ok := cache.Get(ctx, "media1", "1080p", "seg1")
	if !ok {
		t.Error("expected cache hit")
	}
	if string(data) != "test data" {
		t.Errorf("expected data to match, got '%s'", string(data))
	}
}

func TestSegmentCacheCreateDirError(t *testing.T) {
	config := SegmentCacheConfig{
		DiskCacheDir: "/invalid/path/that/does/not/exist/and/cannot/be/created",
	}

	_, err := NewSegmentCache(config)
	if err == nil {
		t.Error("expected error when disk cache dir cannot be created")
	}
}

func TestSegmentCacheCleanup(t *testing.T) {
	cache, err := NewSegmentCache(SegmentCacheConfig{
		MemoryCacheSize: 1 << 20,
		DefaultTTL:      1 * time.Millisecond, // Very short TTL
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	ctx := context.Background()
	cache.Set(ctx, "media1", "1080p", "seg1", []byte("data"))

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	// Trigger cleanup
	cache.cleanupExpired()

	// Entry should be gone
	_, ok := cache.Get(ctx, "media1", "1080p", "seg1")
	if ok {
		t.Error("expected entry to be cleaned up after TTL")
	}

	cache.Close()
}
