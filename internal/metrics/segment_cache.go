// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package metrics

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SegmentCache provides L1 (memory) and L2 (disk) caching for transcoded segments
type SegmentCache struct {
	// L1: In-memory cache for hot segments
	l1        map[string]*L1Entry
	l1Mu      sync.RWMutex
	l1List    *list.List // LRU tracking
	l1MaxSize int64     // Max memory cache size in bytes
	l1MaxItems int       // Max items in memory
	l1Size    int64     // Current memory cache size

	// L2: Disk cache for larger segments
	l2Dir     string
	l2MaxSize int64 // Max disk cache size
	l2Size    int64 // Current disk cache size

	// TTL configuration
	defaultTTL time.Duration

	// Eviction callback
	onEvict func(key string, data []byte)

	// Metrics
	metrics *Metrics
}

// L1Entry represents a cached segment in L1 memory cache
type L1Entry struct {
	Key       string
	Data      []byte
	Size      int64
	ExpiresAt time.Time
	AccessCount int
	LLElem    *list.Element
}

// SegmentCacheConfig holds segment cache configuration
type SegmentCacheConfig struct {
	// L1 memory cache
	MemoryCacheSize int64 // Max bytes in memory (default 100MB)
	MemoryMaxItems int    // Max items in memory

	// L2 disk cache
	DiskCacheDir string
	DiskCacheSize int64 // Max bytes on disk (default 10GB)

	// TTL
	DefaultTTL time.Duration // Default segment TTL (default 1 hour)
}

// NewSegmentCache creates a new segment cache
func NewSegmentCache(config SegmentCacheConfig) (*SegmentCache, error) {
	if config.MemoryCacheSize == 0 {
		config.MemoryCacheSize = 100 << 20 // 100MB
	}
	if config.MemoryMaxItems == 0 {
		config.MemoryMaxItems = 1000
	}
	if config.DiskCacheSize == 0 {
		config.DiskCacheSize = 10 << 30 // 10GB
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = time.Hour
	}

	cache := &SegmentCache{
		l1:          make(map[string]*L1Entry),
		l1List:      list.New(),
		l1MaxSize:   config.MemoryCacheSize,
		l1MaxItems:  config.MemoryMaxItems,
		l2Dir:       config.DiskCacheDir,
		l2MaxSize:   config.DiskCacheSize,
		defaultTTL:  config.DefaultTTL,
		metrics:     Get(),
	}

	// Initialize disk cache directory
	if cache.l2Dir != "" {
		if err := os.MkdirAll(cache.l2Dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create disk cache dir: %w", err)
		}

		// Start cleanup goroutine for expired segments
		go cache.cleanupLoop()
	}

	return cache, nil
}

// Get retrieves a segment from cache (checks L1 first, then L2)
func (sc *SegmentCache) Get(ctx context.Context, mediaID, variant, segmentID string) ([]byte, bool) {
	key := sc.makeKey(mediaID, variant, segmentID)

	// Check L1 first
	if data, ok := sc.getL1(key); ok {
		sc.metrics.RecordCacheHit()
		return data, true
	}

	// Check L2
	if data, ok := sc.getL2(key); ok {
		// Promote to L1
		sc.setL1(key, data)
		sc.metrics.RecordCacheHit()
		return data, true
	}

	sc.metrics.RecordCacheMiss()
	return nil, false
}

// Set stores a segment in cache (L1 + L2)
func (sc *SegmentCache) Set(ctx context.Context, mediaID, variant, segmentID string, data []byte) error {
	key := sc.makeKey(mediaID, variant, segmentID)

	// Store in L1
	sc.setL1(key, data)

	// Store in L2
	if sc.l2Dir != "" {
		return sc.setL2(key, data)
	}

	return nil
}

// Invalidate removes a segment from cache
func (sc *SegmentCache) Invalidate(mediaID, variant, segmentID string) {
	key := sc.makeKey(mediaID, variant, segmentID)
	sc.deleteL1(key)
	sc.deleteL2(key)
}

// InvalidateMedia removes all segments for a media item
func (sc *SegmentCache) InvalidateMedia(mediaID string) {
	sc.l1Mu.Lock()
	defer sc.l1Mu.Unlock()

	prefix := mediaID + ":"
	for key := range sc.l1 {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(sc.l1, key)
		}
	}

	// Note: L2 invalidation would require directory enumeration
}

// makeKey creates a cache key
func (sc *SegmentCache) makeKey(mediaID, variant, segmentID string) string {
	combined := fmt.Sprintf("%s:%s:%s", mediaID, variant, segmentID)
	hash := sha256.Sum256([]byte(combined))
	return mediaID + ":" + hex.EncodeToString(hash[:8])
}

// L1 cache operations

func (sc *SegmentCache) getL1(key string) ([]byte, bool) {
	sc.l1Mu.RLock()
	entry, ok := sc.l1[key]
	sc.l1Mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		sc.deleteL1(key)
		return nil, false
	}

	// Update LRU
	sc.l1Mu.Lock()
	if entry.LLElem != nil {
		sc.l1List.MoveToFront(entry.LLElem)
	}
	entry.AccessCount++
	sc.l1Mu.Unlock()

	return entry.Data, true
}

func (sc *SegmentCache) setL1(key string, data []byte) {
	size := int64(len(data))

	sc.l1Mu.Lock()
	defer sc.l1Mu.Unlock()

	// Evict if necessary
	for (sc.l1Size+size > sc.l1MaxSize || len(sc.l1) >= sc.l1MaxItems) && sc.l1List.Len() > 0 {
		sc.evictL1()
	}

	// Check if already exists
	if entry, ok := sc.l1[key]; ok {
		sc.l1Size -= entry.Size
		entry.Data = data
		entry.Size = size
		entry.ExpiresAt = time.Now().Add(sc.defaultTTL)
		entry.AccessCount++
		if entry.LLElem != nil {
			sc.l1List.MoveToFront(entry.LLElem)
		}
	} else {
		elem := sc.l1List.PushFront(&L1Entry{
			Key:       key,
			Data:      data,
			Size:      size,
			ExpiresAt: time.Now().Add(sc.defaultTTL),
			AccessCount: 1,
		})
		sc.l1[key] = &L1Entry{
			Key:       key,
			Data:      data,
			Size:      size,
			ExpiresAt: time.Now().Add(sc.defaultTTL),
			AccessCount: 1,
			LLElem:    elem,
		}
	}

	sc.l1Size += size
}

func (sc *SegmentCache) deleteL1(key string) {
	sc.l1Mu.Lock()
	defer sc.l1Mu.Unlock()

	if entry, ok := sc.l1[key]; ok {
		sc.l1Size -= entry.Size
		if entry.LLElem != nil {
			sc.l1List.Remove(entry.LLElem)
		}
		delete(sc.l1, key)
	}
}

func (sc *SegmentCache) evictL1() {
	if sc.l1List.Len() == 0 {
		return
	}

	elem := sc.l1List.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*L1Entry)
	sc.l1Size -= entry.Size
	delete(sc.l1, entry.Key)
	sc.l1List.Remove(elem)

	// Notify eviction callback
	if sc.onEvict != nil {
		sc.onEvict(entry.Key, entry.Data)
	}
}

// L2 cache operations

func (sc *SegmentCache) getL2(key string) ([]byte, bool) {
	if sc.l2Dir == "" {
		return nil, false
	}

	path := sc.l2Path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	// Check if expired (L2 stores .meta file with expiry)
	metaPath := path + ".meta"
	if info, err := os.Stat(metaPath); err == nil {
		if time.Now().After(info.ModTime().Add(sc.defaultTTL)) {
			os.Remove(path)
			os.Remove(metaPath)
			return nil, false
		}
	}

	return data, true
}

func (sc *SegmentCache) setL2(key string, data []byte) error {
	if sc.l2Dir == "" {
		return nil
	}

	// Evict if necessary
	for sc.l2Size+int64(len(data)) > sc.l2MaxSize {
		if !sc.evictL2() {
			break
		}
	}

	path := sc.l2Path(key)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write segment to disk cache: %w", err)
	}

	// Write meta file
	metaPath := path + ".meta"
	expiresAt := time.Now().Add(sc.defaultTTL)
	if err := os.WriteFile(metaPath, []byte(expiresAt.Format(time.RFC3339)), 0644); err != nil {
		os.Remove(path)
		return fmt.Errorf("failed to write segment meta: %w", err)
	}

	sc.l2Size += int64(len(data))
	return nil
}

func (sc *SegmentCache) deleteL2(key string) {
	if sc.l2Dir == "" {
		return
	}

	path := sc.l2Path(key)
	if info, err := os.Stat(path); err == nil {
		sc.l2Size -= info.Size()
	}
	os.Remove(path)
	os.Remove(path + ".meta")
}

func (sc *SegmentCache) l2Path(key string) string {
	return filepath.Join(sc.l2Dir, key+".seg")
}

func (sc *SegmentCache) evictL2() bool {
	if sc.l2Dir == "" {
		return false
	}

	// Find oldest .meta file
	entries, err := os.ReadDir(sc.l2Dir)
	if err != nil || len(entries) == 0 {
		return false
	}

	var oldest string
	var oldestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".meta" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if oldestTime.IsZero() || info.ModTime().Before(oldestTime) {
			oldest = entry.Name()
			oldestTime = info.ModTime()
		}
	}

	if oldest == "" {
		return false
	}

	// Remove segment and meta files
	segmentFile := oldest[:len(oldest)-5] // Remove .meta
	segPath := filepath.Join(sc.l2Dir, segmentFile)
	if info, err := os.Stat(segPath); err == nil {
		sc.l2Size -= info.Size()
		os.Remove(segPath)
	}
	os.Remove(filepath.Join(sc.l2Dir, oldest))
	return true
}

// cleanupLoop periodically cleans up expired segments
func (sc *SegmentCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sc.cleanupExpired()
	}
}

// cleanupExpired removes expired segments
func (sc *SegmentCache) cleanupExpired() {
	// Clean L1
	sc.l1Mu.Lock()
	now := time.Now()
	for key, entry := range sc.l1 {
		if now.After(entry.ExpiresAt) {
			sc.l1Size -= entry.Size
			if entry.LLElem != nil {
				sc.l1List.Remove(entry.LLElem)
			}
			delete(sc.l1, key)
		}
	}
	sc.l1Mu.Unlock()

	// Clean L2 (simple - just check for expired .meta files)
	if sc.l2Dir == "" {
		return
	}

	entries, _ := os.ReadDir(sc.l2Dir)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".meta" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// If meta file is older than TTL, segment is expired
		if now := time.Now(); info.ModTime().Add(sc.defaultTTL).Before(now) {
			segmentFile := entry.Name()[:len(entry.Name())-5]
			segPath := filepath.Join(sc.l2Dir, segmentFile)
			if sinfo, err := os.Stat(segPath); err == nil {
				sc.l2Size -= sinfo.Size()
			}
			os.Remove(segPath)
			os.Remove(filepath.Join(sc.l2Dir, entry.Name()))
		}
	}

	// Update metrics
	if sc.metrics != nil {
		sc.metrics.SetCacheSize(sc.l1Size + sc.l2Size)
	}
}

// Stats returns cache statistics
func (sc *SegmentCache) Stats() SegmentCacheStats {
	sc.l1Mu.RLock()
	l1Count := len(sc.l1)
	l1Size := sc.l1Size
	sc.l1Mu.RUnlock()

	return SegmentCacheStats{
		L1Count: l1Count,
		L1Size:  l1Size,
		L1Max:   sc.l1MaxSize,
		L2Count: 0, // Would need L2 enumeration
		L2Size:  sc.l2Size,
		L2Max:   sc.l2MaxSize,
	}
}

// SegmentCacheStats holds cache statistics
type SegmentCacheStats struct {
	L1Count int   `json:"l1_count"`
	L1Size  int64 `json:"l1_size"`
	L1Max   int64 `json:"l1_max"`
	L2Count int   `json:"l2_count"`
	L2Size  int64 `json:"l2_size"`
	L2Max   int64 `json:"l2_max"`
}

// Close releases cache resources
func (sc *SegmentCache) Close() error {
	// Clear L1
	sc.l1Mu.Lock()
	sc.l1 = make(map[string]*L1Entry)
	sc.l1Size = 0
	sc.l1List.Init()
	sc.l1Mu.Unlock()

	return nil
}

// SetOnEvict sets a callback for evicted entries
func (sc *SegmentCache) SetOnEvict(fn func(key string, data []byte)) {
	sc.onEvict = fn
}

// StreamToCache streams a segment directly to cache (for large segments)
func (sc *SegmentCache) StreamToCache(ctx context.Context, mediaID, variant, segmentID string, reader io.Reader) error {
	// Read into buffer
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read segment: %w", err)
	}

	// Store in cache
	return sc.Set(ctx, mediaID, variant, segmentID, data)
}
