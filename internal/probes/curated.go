// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CuratedDevice represents a curated device profile
type CuratedDevice struct {
	ID             string              `json:"id"`
	DeviceHash     string              `json:"device_hash"`
	Name           string              `json:"name"`
	Manufacturer   string              `json:"manufacturer"`
	Model          string              `json:"model"`
	Platform       string              `json:"platform"`
	OSVersions     []string            `json:"os_versions,omitempty"`
	Capabilities   *DeviceCapabilities `json:"capabilities"`
	RecommendedProfile string          `json:"recommended_profile,omitempty"`
	KnownIssues    []KnownIssue        `json:"known_issues,omitempty"`
	Source         string              `json:"source"` // "community", "official", "curated"
	VotesUp        int                 `json:"votes_up"`
	VotesDown      int                 `json:"votes_down"`
	Verified       bool                `json:"verified"`
	Notes          string              `json:"notes,omitempty"`
	LastUpdated    time.Time           `json:"last_updated"`
	CreatedAt      time.Time           `json:"created_at"`
}

// KnownIssue represents a known issue with a device
type KnownIssue struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"` // "info", "warning", "error"
	Codecs      []string `json:"codecs,omitempty"`
	Containers  []string `json:"containers,omitempty"`
	Workaround  string   `json:"workaround,omitempty"`
	Resolved    bool     `json:"resolved"`
}

// CuratedDatabase holds the curated device database
type CuratedDatabase struct {
	devices    map[string]*CuratedDevice
	hashIndex  map[string][]string // device_hash -> device IDs
	platformIndex map[string][]string // platform -> device IDs
	mu         sync.RWMutex

	// Statistics
	stats DatabaseStats

	// Configuration
	dataDir string
}

// DatabaseStats holds database statistics
type DatabaseStats struct {
	TotalDevices     int64 `json:"total_devices"`
	VerifiedDevices  int64 `json:"verified_devices"`
	CommunityDevices int64 `json:"community_devices"`
	OfficialDevices  int64 `json:"official_devices"`
	CuratedDevices   int64 `json:"curated_devices"`
	PlatformsCount   int   `json:"platforms_count"`
}

// NewCuratedDatabase creates a new curated database
func NewCuratedDatabase() *CuratedDatabase {
	return &CuratedDatabase{
		devices:       make(map[string]*CuratedDevice),
		hashIndex:     make(map[string][]string),
		platformIndex: make(map[string][]string),
	}
}

// Load loads curated devices from a directory
func (cd *CuratedDatabase) Load(dataDir string) error {
	cd.dataDir = dataDir

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		slog.Warn("Curated devices directory does not exist", "path", dataDir)
		return nil
	}

	// Walk through the directory and load JSON files
	err := filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(strings.ToLower(path), ".json") {
			if err := cd.loadFromFile(path); err != nil {
				slog.Error("Failed to load curated device file", "path", path, "error", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk curated directory: %w", err)
	}

	cd.calculateStats()
	slog.Info("Loaded curated devices", "count", len(cd.devices))

	return nil
}

// loadFromFile loads curated devices from a single JSON file
func (cd *CuratedDatabase) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Try to parse as single device or array of devices
	var singleDevice CuratedDevice
	var devices []CuratedDevice

	if err := json.Unmarshal(data, &singleDevice); err == nil && singleDevice.ID != "" {
		devices = []CuratedDevice{singleDevice}
	} else if err := json.Unmarshal(data, &devices); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	for i := range devices {
		if err := cd.addDevice(&devices[i]); err != nil {
			slog.Warn("Failed to add device from file", "path", path, "error", err)
		}
	}

	return nil
}

// addDevice adds a device to the database
func (cd *CuratedDatabase) addDevice(device *CuratedDevice) error {
	if device == nil {
		return fmt.Errorf("device is nil")
	}

	if device.ID == "" {
		device.ID = generateDeviceID(device)
	}

	if device.DeviceHash == "" {
		device.DeviceHash = generateDeviceHash(device)
	}

	if device.CreatedAt.IsZero() {
		device.CreatedAt = time.Now()
	}

	if device.LastUpdated.IsZero() {
		device.LastUpdated = time.Now()
	}

	cd.mu.Lock()
	defer cd.mu.Unlock()

	// Check for existing device with same hash
	if existing, ok := cd.devices[device.DeviceHash]; ok {
		// Keep the one with higher vote count or verified status
		if device.Verified || device.VotesUp > existing.VotesUp {
			cd.devices[device.ID] = device
			cd.updateIndexes(device)
		}
		return nil
	}

	cd.devices[device.ID] = device
	cd.updateIndexes(device)

	return nil
}

// updateIndexes updates the hash and platform indexes
func (cd *CuratedDatabase) updateIndexes(device *CuratedDevice) {
	// Update hash index
	if device.DeviceHash != "" {
		cd.hashIndex[device.DeviceHash] = append(cd.hashIndex[device.DeviceHash], device.ID)
	}

	// Update platform index
	platform := strings.ToLower(device.Platform)
	cd.platformIndex[platform] = append(cd.platformIndex[platform], device.ID)
}

// GetByDeviceHash retrieves a device by its hash
func (cd *CuratedDatabase) GetByDeviceHash(deviceHash string) (*CuratedDevice, bool) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	deviceIDs, ok := cd.hashIndex[deviceHash]
	if !ok || len(deviceIDs) == 0 {
		return nil, false
	}

	// Return the first (and typically only) device
	device, ok := cd.devices[deviceIDs[0]]
	return device, ok
}

// GetByPlatform retrieves all devices for a platform
func (cd *CuratedDatabase) GetByPlatform(platform string) []*CuratedDevice {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	platform = strings.ToLower(platform)
	deviceIDs, ok := cd.platformIndex[platform]
	if !ok {
		return nil
	}

	devices := make([]*CuratedDevice, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		if device, ok := cd.devices[id]; ok {
			devices = append(devices, device)
		}
	}

	return devices
}

// GetByID retrieves a device by ID
func (cd *CuratedDatabase) GetByID(id string) (*CuratedDevice, bool) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	device, ok := cd.devices[id]
	return device, ok
}

// Search searches for devices by criteria
func (cd *CuratedDatabase) Search(criteria SearchCriteria) []*CuratedDevice {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	var results []*CuratedDevice

	for _, device := range cd.devices {
		if matchesCriteria(device, criteria) {
			results = append(results, device)
		}
	}

	// Sort by votes and verified status
	sortDevices(results)

	return results
}

// SearchCriteria holds search criteria
type SearchCriteria struct {
	Platform       string
	Manufacturer   string
	Model          string
	VideoCodec     string
	AudioCodec     string
	HasHDR         *bool
	HasDolbyVision *bool
	VerifiedOnly   bool
	Source         string
}

// matchesCriteria checks if a device matches the search criteria
func matchesCriteria(device *CuratedDevice, criteria SearchCriteria) bool {
	if criteria.Platform != "" && !strings.EqualFold(device.Platform, criteria.Platform) {
		return false
	}

	if criteria.Manufacturer != "" && !strings.EqualFold(device.Manufacturer, criteria.Manufacturer) {
		return false
	}

	if criteria.Model != "" && !strings.Contains(strings.ToLower(device.Model), strings.ToLower(criteria.Model)) {
		return false
	}

	if criteria.VideoCodec != "" {
		hasCodec := false
		for _, vc := range device.Capabilities.VideoCodecs {
			if strings.EqualFold(vc, criteria.VideoCodec) {
				hasCodec = true
				break
			}
		}
		if !hasCodec {
			return false
		}
	}

	if criteria.AudioCodec != "" {
		hasCodec := false
		for _, ac := range device.Capabilities.AudioCodecs {
			if strings.EqualFold(ac, criteria.AudioCodec) {
				hasCodec = true
				break
			}
		}
		if !hasCodec {
			return false
		}
	}

	if criteria.HasHDR != nil && *criteria.HasHDR != device.Capabilities.SupportsHDR {
		return false
	}

	if criteria.HasDolbyVision != nil && *criteria.HasDolbyVision != device.Capabilities.SupportsDolbyVision {
		return false
	}

	if criteria.VerifiedOnly && !device.Verified {
		return false
	}

	if criteria.Source != "" && !strings.EqualFold(device.Source, criteria.Source) {
		return false
	}

	return true
}

// sortDevices sorts devices by votes and verified status
func sortDevices(devices []*CuratedDevice) {
	for i := 0; i < len(devices)-1; i++ {
		for j := i + 1; j < len(devices); j++ {
			shouldSwap := false

			// Verified devices first
			if devices[i].Verified && !devices[j].Verified {
				shouldSwap = false
			} else if !devices[i].Verified && devices[j].Verified {
				shouldSwap = true
			} else {
				// Then by vote count
				scoreI := devices[i].VotesUp - devices[i].VotesDown
				scoreJ := devices[j].VotesUp - devices[j].VotesDown
				if scoreI < scoreJ {
					shouldSwap = true
				}
			}

			if shouldSwap {
				devices[i], devices[j] = devices[j], devices[i]
			}
		}
	}
}

// GetStats returns database statistics
func (cd *CuratedDatabase) GetStats() DatabaseStats {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	return cd.stats
}

// calculateStats calculates database statistics
func (cd *CuratedDatabase) calculateStats() {
	stats := DatabaseStats{
		TotalDevices: int64(len(cd.devices)),
	}

	platforms := make(map[string]bool)

	for _, device := range cd.devices {
		if device.Verified {
			stats.VerifiedDevices++
		}

		switch strings.ToLower(device.Source) {
		case "community":
			stats.CommunityDevices++
		case "official":
			stats.OfficialDevices++
		case "curated":
			stats.CuratedDevices++
		}

		platforms[strings.ToLower(device.Platform)] = true
	}

	stats.PlatformsCount = len(platforms)
	cd.stats = stats
}

// AddDevice adds a new device to the database
func (cd *CuratedDatabase) AddDevice(device *CuratedDevice) error {
	if device == nil {
		return fmt.Errorf("device is nil")
	}

	cd.mu.Lock()
	defer cd.mu.Unlock()

	device.ID = generateDeviceID(device)
	device.DeviceHash = generateDeviceHash(device)
	device.CreatedAt = time.Now()
	device.LastUpdated = time.Now()

	cd.devices[device.ID] = device
	cd.updateIndexes(device)
	cd.calculateStats()

	return nil
}

// UpdateDevice updates an existing device
func (cd *CuratedDatabase) UpdateDevice(device *CuratedDevice) error {
	if device == nil || device.ID == "" {
		return fmt.Errorf("invalid device")
	}

	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, ok := cd.devices[device.ID]; !ok {
		return fmt.Errorf("device not found: %s", device.ID)
	}

	device.LastUpdated = time.Now()
	cd.devices[device.ID] = device

	return nil
}

// RemoveDevice removes a device from the database
func (cd *CuratedDatabase) RemoveDevice(deviceID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	device, ok := cd.devices[deviceID]
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	// Remove from indexes
	if device.DeviceHash != "" {
		hashIDs := cd.hashIndex[device.DeviceHash]
		for i, id := range hashIDs {
			if id == deviceID {
				cd.hashIndex[device.DeviceHash] = append(hashIDs[:i], hashIDs[i+1:]...)
				break
			}
		}
	}

	platform := strings.ToLower(device.Platform)
	platformIDs := cd.platformIndex[platform]
	for i, id := range platformIDs {
		if id == deviceID {
			cd.platformIndex[platform] = append(platformIDs[:i], platformIDs[i+1:]...)
			break
		}
	}

	delete(cd.devices, deviceID)
	cd.calculateStats()

	return nil
}

// Vote casts a vote for a device
func (cd *CuratedDatabase) Vote(deviceID string, up bool) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	device, ok := cd.devices[deviceID]
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	if up {
		device.VotesUp++
	} else {
		device.VotesDown++
	}

	device.LastUpdated = time.Now()
	cd.devices[deviceID] = device

	return nil
}

// MarkVerified marks a device as verified
func (cd *CuratedDatabase) MarkVerified(deviceID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	device, ok := cd.devices[deviceID]
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	device.Verified = true
	device.LastUpdated = time.Now()
	cd.devices[deviceID] = device

	return nil
}

// GetRecommendedProfile returns the recommended quality profile for a device
func (cd *CuratedDatabase) GetRecommendedProfile(device *CuratedDevice) string {
	if device == nil || device.Capabilities == nil {
		return ""
	}

	// If already set, return it
	if device.RecommendedProfile != "" {
		return device.RecommendedProfile
	}

	// Determine profile based on capabilities
	caps := device.Capabilities

	// Check for 4K UHD support
	if caps.MaxWidth >= 3840 && caps.MaxHeight >= 2160 {
		if caps.SupportsHDR || caps.SupportsDolbyVision {
			return "ultra_hd_hdr"
		}
		return "ultra_hd"
	}

	// Check for Full HD support
	if caps.MaxWidth >= 1920 && caps.MaxHeight >= 1080 {
		if caps.SupportsHDR {
			return "full_hd_hdr"
		}
		return "full_hd"
	}

	// Check for HD support
	if caps.MaxWidth >= 1280 && caps.MaxHeight >= 720 {
		return "hd"
	}

	// Default to SD
	return "sd"
}

// GetKnownIssues returns known issues for a device
func (cd *CuratedDatabase) GetKnownIssues(device *CuratedDevice) []KnownIssue {
	if device == nil {
		return nil
	}

	return device.KnownIssues
}

// HasKnownIssues checks if a device has known issues
func (cd *CuratedDatabase) HasKnownIssues(device *CuratedDevice) bool {
	return len(cd.GetKnownIssues(device)) > 0
}

// GetWorkaround returns a workaround for a known issue
func (cd *CuratedDatabase) GetWorkaround(device *CuratedDevice, issueCode string) string {
	issues := cd.GetKnownIssues(device)
	for _, issue := range issues {
		if issue.ID == issueCode {
			return issue.Workaround
		}
	}
	return ""
}

// GetAll returns all devices in the database
func (cd *CuratedDatabase) GetAll() []*CuratedDevice {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	devices := make([]*CuratedDevice, 0, len(cd.devices))
	for _, device := range cd.devices {
		devices = append(devices, device)
	}

	sortDevices(devices)
	return devices
}

// Count returns the total number of devices
func (cd *CuratedDatabase) Count() int {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return len(cd.devices)
}

// GenerateDeviceID generates a unique device ID
func generateDeviceID(device *CuratedDevice) string {
	return fmt.Sprintf("%s-%s-%s-%d",
		strings.ToLower(device.Manufacturer),
		strings.ToLower(device.Model),
		strings.ToLower(device.Platform),
		time.Now().UnixNano(),
	)
}

// GenerateDeviceHash generates a consistent hash for device matching
func generateDeviceHash(device *CuratedDevice) string {
	// Create a consistent hash based on device characteristics
	hashParts := []string{
		strings.ToLower(device.Platform),
		strings.ToLower(device.Manufacturer),
		strings.ToLower(device.Model),
	}

	// Add codec info for more specific matching
	if device.Capabilities != nil {
		for _, vc := range device.Capabilities.VideoCodecs {
			hashParts = append(hashParts, "v:"+vc)
		}
		for _, ac := range device.Capabilities.AudioCodecs {
			hashParts = append(hashParts, "a:"+ac)
		}
	}

	return strings.Join(hashParts, "|")
}

// LoadFromEmbedded loads devices from embedded JSON data
func (cd *CuratedDatabase) LoadFromEmbedded(deviceJSON []byte) error {
	var devices []CuratedDevice
	if err := json.Unmarshal(deviceJSON, &devices); err != nil {
		return fmt.Errorf("failed to unmarshal embedded devices: %w", err)
	}

	cd.mu.Lock()
	defer cd.mu.Unlock()

	for i := range devices {
		device := &devices[i]
		cd.devices[device.ID] = device
		if device.DeviceHash != "" {
			cd.hashIndex[device.DeviceHash] = append(cd.hashIndex[device.DeviceHash], device.ID)
		}
		if device.Platform != "" {
			cd.platformIndex[device.Platform] = append(cd.platformIndex[device.Platform], device.ID)
		}
		cd.stats.TotalDevices++
		if device.Verified {
			cd.stats.VerifiedDevices++
		}
	}

	cd.stats.PlatformsCount = len(cd.platformIndex)
	return nil
}