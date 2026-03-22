// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// CapabilityCache provides in-memory and SQLite caching for device capabilities
type CapabilityCache struct {
	// In-memory cache
	memoryCache map[string]*CachedCapability
	mu          sync.RWMutex

	// SQLite backend
	db *sql.DB

	// Configuration
	memoryTTL      time.Duration
	sqliteTTL      time.Duration
	enableSQLite   bool
	maxMemoryItems int

	// Statistics
	stats CacheStats
}

// CachedCapability represents a cached capability entry
type CachedCapability struct {
	Capabilities *DeviceCapabilities `json:"capabilities"`
	CreatedAt    time.Time           `json:"created_at"`
	ExpiresAt    time.Time           `json:"expires_at"`
	AccessCount  int                 `json:"access_count"`
	LastAccessed time.Time           `json:"last_accessed"`
	Source       string              `json:"source"` // "probe", "curated", "manual"
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits         int64 `json:"hits"`
	Misses       int64 `json:"misses"`
	MemorySize   int   `json:"memory_size"`
	SQLiteSize   int   `json:"sqlite_size"`
	Evictions    int64 `json:"evictions"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	MemoryTTL      time.Duration `json:"memory_ttl"`
	SQLiteTTL      time.Duration `json:"sqlite_ttl"`
	EnableSQLite   bool          `json:"enable_sqlite"`
	MaxMemoryItems int           `json:"max_memory_items"`
	DatabasePath   string        `json:"database_path"`
}

// NewCapabilityCache creates a new capability cache
func NewCapabilityCache(config *CacheConfig) (*CapabilityCache, error) {
	if config == nil {
		config = &CacheConfig{
			MemoryTTL:      time.Hour * 24,
			SQLiteTTL:      time.Hour * 24 * 30,
			EnableSQLite:   false,
			MaxMemoryItems: 10000,
		}
	}

	// Apply defaults for zero values
	memTTL := config.MemoryTTL
	if memTTL == 0 {
		memTTL = time.Hour * 24
	}
	maxItems := config.MaxMemoryItems
	if maxItems == 0 {
		maxItems = 10000
	}

	cache := &CapabilityCache{
		memoryCache:    make(map[string]*CachedCapability),
		memoryTTL:      memTTL,
		sqliteTTL:      config.SQLiteTTL,
		enableSQLite:   config.EnableSQLite,
		maxMemoryItems: maxItems,
	}

	// Initialize SQLite backend if enabled
	if config.EnableSQLite {
		if err := cache.initSQLite(config.DatabasePath); err != nil {
			return nil, fmt.Errorf("failed to initialize SQLite cache: %w", err)
		}
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache, nil
}

// initSQLite initializes the SQLite backend
func (c *CapabilityCache) initSQLite(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db

	// Create table if not exists
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS device_capabilities_cache (
		device_id TEXT PRIMARY KEY,
		capabilities TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'probe',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		access_count INTEGER NOT NULL DEFAULT 0,
		last_accessed TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_cache_expires ON device_capabilities_cache(expires_at);
	CREATE INDEX IF NOT EXISTS idx_cache_source ON device_capabilities_cache(source);
	CREATE INDEX IF NOT EXISTS idx_cache_access ON device_capabilities_cache(access_count);
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to create cache table: %w", err)
	}

	return nil
}

// Get retrieves capabilities from cache
func (c *CapabilityCache) Get(deviceID string) (*DeviceCapabilities, bool) {
	return c.GetWithContext(context.Background(), deviceID)
}

// GetWithContext retrieves capabilities from cache with context support
func (c *CapabilityCache) GetWithContext(ctx context.Context, deviceID string) (*DeviceCapabilities, bool) {
	if deviceID == "" {
		return nil, false
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, false
	default:
	}

	// Check memory cache first
	c.mu.RLock()
	cached, found := c.memoryCache[deviceID]
	c.mu.RUnlock()

	if found && !c.isExpired(cached) {
		c.recordHit()
		c.updateAccess(cached)
		return cached.Capabilities, true
	}

	// Fall back to SQLite
	if c.enableSQLite && c.db != nil {
		// Pass context to SQLite query
		return c.getFromSQLiteWithContext(ctx, deviceID)
	}

	c.recordMiss()
	return nil, false
}

// getFromSQLite retrieves capabilities from SQLite
func (c *CapabilityCache) getFromSQLite(deviceID string) (*DeviceCapabilities, bool) {
	query := `SELECT capabilities, source, created_at, access_count, last_accessed
			  FROM device_capabilities_cache
			  WHERE device_id = ? AND expires_at > datetime('now')
			  LIMIT 1`

	var capabilitiesJSON string
	var source string
	var createdAt, lastAccessed time.Time
	var accessCount int

	err := c.db.QueryRow(query, deviceID).Scan(
		&capabilitiesJSON, &source, &createdAt, &accessCount, &lastAccessed,
	)

	if err == sql.ErrNoRows {
		c.recordMiss()
		return nil, false
	}

	if err != nil {
		return nil, false
	}

	var caps DeviceCapabilities
	if err := json.Unmarshal([]byte(capabilitiesJSON), &caps); err != nil {
		return nil, false
	}

	cached := &CachedCapability{
		Capabilities: &caps,
		CreatedAt:    createdAt,
		ExpiresAt:    time.Now().Add(c.memoryTTL),
		AccessCount:  accessCount,
		LastAccessed: lastAccessed,
		Source:       source,
	}

	c.mu.Lock()
	c.memoryCache[deviceID] = cached
	c.mu.Unlock()

	c.recordHit()
	return &caps, true
}

// getFromSQLiteWithContext retrieves capabilities from SQLite with context support
func (c *CapabilityCache) getFromSQLiteWithContext(ctx context.Context, deviceID string) (*DeviceCapabilities, bool) {
	query := `SELECT capabilities, source, created_at, access_count, last_accessed
			  FROM device_capabilities_cache
			  WHERE device_id = ? AND expires_at > datetime('now')
			  LIMIT 1`

	var capabilitiesJSON string
	var source string
	var createdAt, lastAccessed time.Time
	var accessCount int

	// Use context-aware query
	var row *sql.Row
	if c.db != nil {
		// For SQLite, we use QueryRow directly but could use context if using context-aware driver
		row = c.db.QueryRowContext(ctx, query, deviceID)
	} else {
		return nil, false
	}

	err := row.Scan(
		&capabilitiesJSON, &source, &createdAt, &accessCount, &lastAccessed,
	)

	if err == sql.ErrNoRows {
		c.recordMiss()
		return nil, false
	}

	if err != nil {
		return nil, false
	}

	var caps DeviceCapabilities
	if err := json.Unmarshal([]byte(capabilitiesJSON), &caps); err != nil {
		return nil, false
	}

	cached := &CachedCapability{
		Capabilities: &caps,
		CreatedAt:    createdAt,
		ExpiresAt:    time.Now().Add(c.memoryTTL),
		AccessCount:  accessCount,
		LastAccessed: lastAccessed,
		Source:       source,
	}

	c.mu.Lock()
	c.memoryCache[deviceID] = cached
	c.mu.Unlock()

	c.recordHit()
	return &caps, true
}

// Set stores capabilities in cache
func (c *CapabilityCache) Set(deviceID string, caps *DeviceCapabilities, source string) error {
	if deviceID == "" || caps == nil {
		return fmt.Errorf("invalid device ID or capabilities")
	}

	now := time.Now()
	expiresAt := now.Add(c.memoryTTL)

	cached := &CachedCapability{
		Capabilities: caps,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		AccessCount:  0,
		LastAccessed: now,
		Source:       source,
	}

	// Store in memory cache
	c.mu.Lock()
	if len(c.memoryCache) >= c.maxMemoryItems {
		c.evictOldest()
	}
	c.memoryCache[deviceID] = cached
	c.mu.Unlock()

	// Store in SQLite if enabled
	if c.enableSQLite && c.db != nil {
		c.saveToSQLite(deviceID, cached)
	}

	return nil
}

// saveToSQLite stores capabilities in SQLite
func (c *CapabilityCache) saveToSQLite(deviceID string, cached *CachedCapability) {
	capabilitiesJSON, err := json.Marshal(cached.Capabilities)
	if err != nil {
		return
	}

	upsertSQL := `
	INSERT OR REPLACE INTO device_capabilities_cache
	(device_id, capabilities, source, created_at, expires_at, access_count, last_accessed)
	VALUES (?, ?, ?, datetime('now'), datetime('now', ?), 0, datetime('now'))
	`

	_, err = c.db.Exec(upsertSQL, deviceID, capabilitiesJSON, cached.Source,
		fmt.Sprintf("+%d seconds", int(c.sqliteTTL.Seconds())))

	if err != nil {
		// Log error but don't fail the operation
		return
	}
}

// Delete removes capabilities from cache
func (c *CapabilityCache) Delete(deviceID string) {
	// Remove from memory cache
	c.mu.Lock()
	delete(c.memoryCache, deviceID)
	c.mu.Unlock()

	// Remove from SQLite
	if c.enableSQLite && c.db != nil {
		_, _ = c.db.Exec("DELETE FROM device_capabilities_cache WHERE device_id = ?", deviceID)
	}
}

// DeleteExpired removes expired entries from cache
func (c *CapabilityCache) DeleteExpired() int {
	deleted := 0

	// Clean memory cache
	c.mu.Lock()
	for deviceID, cached := range c.memoryCache {
		if c.isExpired(cached) {
			delete(c.memoryCache, deviceID)
			deleted++
		}
	}
	c.mu.Unlock()

	// Clean SQLite
	if c.enableSQLite && c.db != nil {
		result, err := c.db.Exec(
			"DELETE FROM device_capabilities_cache WHERE expires_at < datetime('now')",
		)
		if err == nil {
			rows, _ := result.RowsAffected()
			deleted += int(rows)
		}
	}

	c.stats.Evictions += int64(deleted)
	return deleted
}

// isExpired checks if a cached entry has expired
func (c *CapabilityCache) isExpired(cached *CachedCapability) bool {
	return time.Now().After(cached.ExpiresAt)
}

// updateAccess updates access tracking for a cached entry
func (c *CapabilityCache) updateAccess(cached *CachedCapability) {
	cached.AccessCount++
	cached.LastAccessed = time.Now()
}

// recordHit records a cache hit
func (c *CapabilityCache) recordHit() {
	c.stats.Hits++
}

// recordMiss records a cache miss
func (c *CapabilityCache) recordMiss() {
	c.stats.Misses++
}

// evictOldest evicts the oldest entries when memory cache is full
func (c *CapabilityCache) evictOldest() {
	// Find entries to evict (oldest by last accessed)
	type entry struct {
		deviceID     string
		lastAccessed time.Time
	}

	var entries []entry
	for deviceID, cached := range c.memoryCache {
		entries = append(entries, entry{
			deviceID:     deviceID,
			lastAccessed: cached.LastAccessed,
		})
	}

	// Sort by last accessed (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].lastAccessed.Before(entries[i].lastAccessed) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Evict oldest 10% or at least 1 entry
	evictCount := len(entries) / 10
	if evictCount < 1 {
		evictCount = 1
	}
	if evictCount > len(entries) {
		evictCount = len(entries)
	}

	for i := 0; i < evictCount; i++ {
		delete(c.memoryCache, entries[i].deviceID)
		c.stats.Evictions++
	}
}

// cleanupLoop runs periodic cleanup
func (c *CapabilityCache) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.DeleteExpired()
	}
}

// GetStats returns cache statistics
func (c *CapabilityCache) GetStats() CacheStats {
	c.mu.RLock()
	memSize := len(c.memoryCache)
	c.mu.RUnlock()

	stats := c.stats
	stats.MemorySize = memSize

	if c.enableSQLite && c.db != nil {
		var count int
		err := c.db.QueryRow("SELECT COUNT(*) FROM device_capabilities_cache").Scan(&count)
		if err == nil {
			stats.SQLiteSize = count
		}
	}

	return stats
}

// GetMemorySize returns current memory cache size
func (c *CapabilityCache) GetMemorySize() int {
	c.mu.RLock()
	size := len(c.memoryCache)
	c.mu.RUnlock()
	return size
}

// Clear clears all cached entries
func (c *CapabilityCache) Clear() {
	// Clear memory cache
	c.mu.Lock()
	c.memoryCache = make(map[string]*CachedCapability)
	c.mu.Unlock()

	// Clear SQLite
	if c.enableSQLite && c.db != nil {
		_, _ = c.db.Exec("DELETE FROM device_capabilities_cache")
	}

	c.stats = CacheStats{}
}

// Close closes the cache and releases resources
func (c *CapabilityCache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// GetCapabilitiesByPlatform retrieves capabilities for a specific platform
func (c *CapabilityCache) GetCapabilitiesByPlatform(platform string) ([]*DeviceCapabilities, error) {
	if !c.enableSQLite || c.db == nil {
		return nil, fmt.Errorf("SQLite cache not enabled")
	}

	query := `SELECT capabilities
			  FROM device_capabilities_cache
			  WHERE expires_at > datetime('now')
			  AND capabilities LIKE ?
			  LIMIT 100`

	// Search for platform in capabilities JSON
	platformSearch := fmt.Sprintf(`%%"platform": "%s"%%`, platform)

	rows, err := c.db.Query(query, platformSearch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*DeviceCapabilities
	for rows.Next() {
		var capabilitiesJSON string
		if err := rows.Scan(&capabilitiesJSON); err != nil {
			continue
		}

		var caps DeviceCapabilities
		if err := json.Unmarshal([]byte(capabilitiesJSON), &caps); err != nil {
			continue
		}

		results = append(results, &caps)
	}

	return results, rows.Err()
}

// GetCapabilitiesByCodec retrieves capabilities that support a specific codec
func (c *CapabilityCache) GetCapabilitiesByCodec(codec string) ([]*DeviceCapabilities, error) {
	if !c.enableSQLite || c.db == nil {
		return nil, fmt.Errorf("SQLite cache not enabled")
	}

	query := `SELECT capabilities
			  FROM device_capabilities_cache
			  WHERE expires_at > datetime('now')
			  AND capabilities LIKE ?
			  LIMIT 100`

	// Search for codec in video_codecs array
	codecSearch := fmt.Sprintf(`%%"video_codecs": [%%"%s"%%]%%`, codec)

	rows, err := c.db.Query(query, codecSearch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*DeviceCapabilities
	for rows.Next() {
		var capabilitiesJSON string
		if err := rows.Scan(&capabilitiesJSON); err != nil {
			continue
		}

		var caps DeviceCapabilities
		if err := json.Unmarshal([]byte(capabilitiesJSON), &caps); err != nil {
			continue
		}

		results = append(results, &caps)
	}

	return results, rows.Err()
}

// BatchSet stores multiple capabilities entries
func (c *CapabilityCache) BatchSet(entries map[string]*DeviceCapabilities, source string) error {
	for deviceID, caps := range entries {
		if err := c.Set(deviceID, caps, source); err != nil {
			return fmt.Errorf("failed to cache device %s: %w", deviceID, err)
		}
	}
	return nil
}

// BatchGet retrieves multiple capabilities entries
func (c *CapabilityCache) BatchGet(deviceIDs []string) (map[string]*DeviceCapabilities, error) {
	results := make(map[string]*DeviceCapabilities)

	for _, deviceID := range deviceIDs {
		if caps, found := c.Get(deviceID); found {
			results[deviceID] = caps
		}
	}

	return results, nil
}

// UpdateAccessCount increments the access count for a device
func (c *CapabilityCache) UpdateAccessCount(deviceID string) error {
	if !c.enableSQLite || c.db == nil {
		return fmt.Errorf("SQLite cache not enabled")
	}

	_, err := c.db.Exec(
		"UPDATE device_capabilities_cache SET access_count = access_count + 1, last_accessed = datetime('now') WHERE device_id = ?",
		deviceID,
	)
	return err
}

// GetMostAccessed returns the most accessed capabilities
func (c *CapabilityCache) GetMostAccessed(limit int) ([]*CachedCapability, error) {
	if !c.enableSQLite || c.db == nil {
		return nil, fmt.Errorf("SQLite cache not enabled")
	}

	query := `SELECT device_id, capabilities, source, created_at, access_count, last_accessed
			  FROM device_capabilities_cache
			  WHERE expires_at > datetime('now')
			  ORDER BY access_count DESC
			  LIMIT ?`

	rows, err := c.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*CachedCapability
	for rows.Next() {
		var deviceID string
		var capabilitiesJSON string
		var source string
		var createdAt, lastAccessed time.Time
		var accessCount int

		if err := rows.Scan(&deviceID, &capabilitiesJSON, &source, &createdAt, &accessCount, &lastAccessed); err != nil {
			continue
		}

		var caps DeviceCapabilities
		if err := json.Unmarshal([]byte(capabilitiesJSON), &caps); err != nil {
			continue
		}

		results = append(results, &CachedCapability{
			Capabilities: &caps,
			CreatedAt:    createdAt,
			AccessCount:  accessCount,
			LastAccessed: lastAccessed,
			Source:       source,
		})
	}

	return results, rows.Err()
}
