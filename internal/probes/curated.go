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
	"sort"
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
	DeviceClass    string              `json:"device_class"`    // "A", "B", "C", "D" - premium to budget
	Year           int                 `json:"year"`             // Manufacturing year
	SoC            string              `json:"soc"`              // System-on-Chip (e.g., "Exynos M7", "α9 Gen 6", "Amlogic S905X4")
	SoCAliases     []string            `json:"soc_aliases"`     // Alternate SoC names for matching
	OSVersions     []string            `json:"os_versions,omitempty"`
	Capabilities   *DeviceCapabilities  `json:"capabilities"`
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
	codecIndex map[string][]string // video codec -> device IDs
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
		codecIndex:   make(map[string][]string),
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

	// Check for existing device with same hash using the hashIndex
	if existingIDs, ok := cd.hashIndex[device.DeviceHash]; ok && len(existingIDs) > 0 {
		// Get the existing device (first one with matching hash)
		existingID := existingIDs[0]
		if existing, ok := cd.devices[existingID]; ok {
			// Keep the one with higher vote count or verified status
			if device.Verified || device.VotesUp > existing.VotesUp {
				// Update the existing entry instead of adding a new one
				device.ID = existing.ID // Preserve original ID
				device.CreatedAt = existing.CreatedAt // Preserve creation time
				device.VotesUp = existing.VotesUp // Preserve votes
				device.VotesDown = existing.VotesDown
				device.Verified = existing.Verified || device.Verified // Keep verified if set
				cd.devices[existing.ID] = device
				cd.updateIndexes(device)
			}
			return nil
		}
	}

	// No existing device, add as new
	cd.devices[device.ID] = device
	cd.updateIndexes(device)

	return nil
}

// updateIndexes updates the hash, platform, and codec indexes
func (cd *CuratedDatabase) updateIndexes(device *CuratedDevice) {
	// Update hash index
	if device.DeviceHash != "" {
		cd.hashIndex[device.DeviceHash] = append(cd.hashIndex[device.DeviceHash], device.ID)
	}

	// Update platform index
	platform := strings.ToLower(device.Platform)
	cd.platformIndex[platform] = append(cd.platformIndex[platform], device.ID)

	// Update codec index for video codecs
	if device.Capabilities != nil {
		for _, codec := range device.Capabilities.VideoCodecs {
			codecLower := strings.ToLower(codec)
			cd.codecIndex[codecLower] = append(cd.codecIndex[codecLower], device.ID)
		}
	}
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

// GetByCodec retrieves all devices that support a specific video codec
func (cd *CuratedDatabase) GetByCodec(codec string) []*CuratedDevice {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	codec = strings.ToLower(codec)
	deviceIDs, ok := cd.codecIndex[codec]
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
	// Sort by votes and verified status - O(n log n)
	sort.Slice(devices, func(i, j int) bool {
		// Verified devices first
		if devices[i].Verified != devices[j].Verified {
			return devices[i].Verified
		}
		// Then by vote score
		scoreI := devices[i].VotesUp - devices[i].VotesDown
		scoreJ := devices[j].VotesUp - devices[j].VotesDown
		return scoreI > scoreJ
	})
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

// =============================================================================
// VENDOR BEHAVIOR RULES
// Based on DEVICE_DATABASE.md Section 4
// These rules encode platform-specific behavior that overrides capability detection
// =============================================================================

// VendorRule represents a vendor-specific behavior rule
type VendorRule struct {
	Condition   string   // Rule condition (e.g., "year >= 2022")
	HasDTS      bool     `json:"has_dts,omitempty"`
	NoDTS       bool     `json:"no_dts,omitempty"`
	HasDolbyVision    bool     `json:"has_dolby_vision,omitempty"`
	NoDolbyVision     bool     `json:"no_dolby_vision,omitempty"`
	HasHDR10Plus      bool     `json:"has_hdr10_plus,omitempty"`
	NoHDR10Plus       bool     `json:"no_hdr10_plus,omitempty"`
	HasAV1           bool     `json:"has_av1,omitempty"`
	HasTrueHD        bool     `json:"has_truehd,omitempty"`
	HasSSA           bool     `json:"has_ssa,omitempty"`
	HasTTML          bool     `json:"has_ttml,omitempty"`
	HasWebM          bool     `json:"has_webm,omitempty"`
	Workaround string   `json:"workaround,omitempty"`
}

// vendorRules encodes vendor-specific behavior based on DEVICE_DATABASE.md
var vendorRules = map[string][]VendorRule{
	"samsung": {
		// Samsung NEVER supports DTS (rule from DEVICE_DATABASE.md)
		{Condition: "always", NoDTS: true, Workaround: "Transcode DTS to AC-3 or E-AC-3"},
		// Samsung NEVER supports Dolby Vision
		{Condition: "always", NoDolbyVision: true, Workaround: "Use HDR10 or HDR10+ instead"},
		// Samsung never adopted HDR10+ in early models, but 2022+ has it
		{Condition: "year >= 2022", HasHDR10Plus: true},
		// Samsung AV1 support starts 2022 (Neo QLED)
		{Condition: "year >= 2022", HasAV1: true},
		// Samsung supports SSA/ASS subtitles
		{Condition: "always", HasSSA: true},
		// Samsung supports TTML (but not WebM)
		{Condition: "always", HasTTML: true},
		{Condition: "always", NoHDR10Plus: false}, // Reset for older models
	},
	"lg": {
		// LG ALWAYS supports Dolby Vision (rule from DEVICE_DATABASE.md)
		{Condition: "always", HasDolbyVision: true},
		// LG never adopted HDR10+
		{Condition: "always", NoHDR10Plus: true, Workaround: "Use HDR10 or DV instead"},
		// LG AV1 support starts 2022 (α7 Gen 5 / α9 Gen 5)
		{Condition: "year >= 2022", HasAV1: true},
		// LG supports SSA/ASS subtitles
		{Condition: "always", HasSSA: true},
		// LG supports TTML
		{Condition: "always", HasTTML: true},
	},
	"roku": {
		// Roku TV OS behavior (different from raw Roku)
		{Condition: "always", NoDTS: false}, // Some Roku TV models support DTS via TV speakers
		{Condition: "always", HasDolbyVision: false}, // No DV on Roku
		// Roku TV models (TCL, Hisense, etc.) run Roku TV OS
		// They support Dolby Vision on premium models (TCL 6-series)
		{Condition: "model contains 'TCL' or model contains 'Roku TV'", HasDolbyVision: true},
	},
	"amazon": {
		// Amazon Fire TV (rule from DEVICE_DATABASE.md)
		{Condition: "always", NoDTS: true, Workaround: "Transcode DTS to AC-3 or E-AC-3"},
		// Fire TV supports DV Profile 8
		{Condition: "always", HasDolbyVision: true},
		// Fire TV 3rd gen and later support AV1
		{Condition: "year >= 2022", HasAV1: true},
		// No SSA/ASS on Fire TV
		{Condition: "always", NoHDR10Plus: true}, // Fire TV doesn't support HDR10+
		// Fire TV subtitle formats
		{Condition: "always", HasTTML: false},
	},
	"apple": {
		// Apple TV (rule from DEVICE_DATABASE.md)
		{Condition: "always", NoDTS: true, Workaround: "Apple doesn't support DTS passthrough"},
		// Apple supports DV and HDR10
		{Condition: "always", HasDolbyVision: true},
		// Apple supports HDR10+ since tvOS 17
		{Condition: "year >= 2024", HasHDR10Plus: true},
		// Apple has full subtitle support
		{Condition: "always", HasSSA: true, HasTTML: true},
		// Apple supports WebM
		{Condition: "always", HasWebM: true},
	},
	"sony": {
		// Sony Android TV
		{Condition: "always", HasDolbyVision: true},
		// Sony AV1 support starts 2021
		{Condition: "year >= 2021", HasAV1: true},
		// Sony supports DTS
		{Condition: "always", HasTrueHD: true},
		{Condition: "always", HasSSA: true, HasTTML: true},
	},
	"nvidia": {
		// NVIDIA Shield
		{Condition: "always", HasDolbyVision: true},
		{Condition: "always", HasAV1: true},
		{Condition: "always", HasTrueHD: true, NoDTS: false}, // Shield supports DTS
		{Condition: "always", HasSSA: true, HasTTML: true, HasWebM: true},
	},
	"xiaomi": {
		// Xiaomi Mi Box
		{Condition: "always", HasDolbyVision: true},
		// Mi Box S (2018) has no AV1, later models may
		{Condition: "year >= 2022", HasAV1: true},
		// No TrueHD on Mi Box S
		{Condition: "model contains 'Mi Box S'", HasTrueHD: false},
		// SSA/ASS support varies
		{Condition: "model contains 'Mi Box S'", HasSSA: false},
	},
	"philips": {
		// Philips Android TV
		{Condition: "always", HasDolbyVision: true},
		{Condition: "year >= 2022", HasAV1: true},
		// Saphi OS is different (limited codec support)
		{Condition: "model contains 'Saphi'", HasDolbyVision: false},
		{Condition: "model contains 'Saphi'", NoDTS: true},
	},
	"hisense": {
		// Hisense varies by OS
		// Android TV models support DV
		{Condition: "model contains 'Android'", HasDolbyVision: true},
		// Vidaa OS models have limited support
		{Condition: "not model contains 'Android'", NoDolbyVision: true},
		{Condition: "not model contains 'Android'", NoDTS: true},
		{Condition: "year >= 2022", HasAV1: true},
	},
}

// GetVendorRules returns the rules for a given vendor/manufacturer
func GetVendorRules(manufacturer string) []VendorRule {
	manufacturer = strings.ToLower(manufacturer)
	if rules, ok := vendorRules[manufacturer]; ok {
		return rules
	}
	return nil
}

// ApplyVendorRules applies vendor-specific rules to a device's capabilities
func ApplyVendorRules(device *CuratedDevice) {
	if device == nil || device.Capabilities == nil {
		return
	}

	rules := GetVendorRules(device.Manufacturer)
	if rules == nil {
		return
	}

	// Infer year from model name if not set
	year := device.Year
	if year == 0 {
		year = inferYearFromModel(device.Model)
	}

	caps := device.Capabilities

	for _, rule := range rules {
		// Check if rule condition matches
		if !matchesRuleCondition(rule.Condition, device, year) {
			continue
		}

		// Apply DTS rule
		if rule.NoDTS {
			caps.SupportsDTS = false
			// Remove DTS from audio codecs
			caps.AudioCodecs = removeString(caps.AudioCodecs, "dts")
			caps.AudioCodecs = removeString(caps.AudioCodecs, "dtshd")
		}
		if rule.HasDTS && !rule.NoDTS {
			caps.SupportsDTS = true
			if !containsString(caps.AudioCodecs, "dts") {
				caps.AudioCodecs = append(caps.AudioCodecs, "dts")
			}
		}

		// Apply Dolby Vision rule
		if rule.NoDolbyVision {
			caps.SupportsDolbyVision = false
		}
		if rule.HasDolbyVision && !rule.NoDolbyVision {
			caps.SupportsDolbyVision = true
		}

		// Apply HDR10+ rule (only if HDR is supported)
		if rule.HasHDR10Plus && caps.SupportsHDR {
			// HDR10+ is supported - ensure hdr10 is in HDR list
			// Note: this is metadata, not a capability flag in current model
		}
		if rule.NoHDR10Plus {
			// HDR10+ not supported - this doesn't affect HDR flag
			// Just informational
		}

		// Apply AV1 rule
		if rule.HasAV1 {
			if !containsString(caps.VideoCodecs, "av1") {
				caps.VideoCodecs = append(caps.VideoCodecs, "av1")
			}
		}

		// Apply TrueHD rule
		if rule.HasTrueHD {
			if !containsString(caps.AudioCodecs, "truehd") {
				caps.AudioCodecs = append(caps.AudioCodecs, "truehd")
			}
			if !containsString(caps.AudioCodecs, "flac") {
				caps.AudioCodecs = append(caps.AudioCodecs, "flac")
			}
		}

		// Apply SSA/ASS rule
		if rule.HasSSA {
			if !containsString(caps.SubtitleFormats, "ssa") {
				caps.SubtitleFormats = append(caps.SubtitleFormats, "ssa")
			}
			if !containsString(caps.SubtitleFormats, "ass") {
				caps.SubtitleFormats = append(caps.SubtitleFormats, "ass")
			}
		}

		// Apply TTML rule
		if rule.HasTTML {
			if !containsString(caps.SubtitleFormats, "ttml") {
				caps.SubtitleFormats = append(caps.SubtitleFormats, "ttml")
			}
		}

		// Apply WebM rule
		if rule.HasWebM {
			if !containsString(caps.ContainerFormats, "webm") {
				caps.ContainerFormats = append(caps.ContainerFormats, "webm")
			}
		}
	}
}

// matchesRuleCondition checks if a rule condition matches the device
func matchesRuleCondition(condition string, device *CuratedDevice, year int) bool {
	switch {
	case condition == "always":
		return true
	case strings.HasPrefix(condition, "year >= "):
		requiredYear := 0
		fmt.Sscanf(condition, "year >= %d", &requiredYear)
		return year >= requiredYear
	case strings.HasPrefix(condition, "not "):
		// Negation - check if condition is false
		inner := strings.TrimPrefix(condition, "not ")
		return !matchesRuleCondition(inner, device, year)
	case strings.Contains(condition, "model contains "):
		modelPart := strings.TrimPrefix(condition, "model contains ")
		modelPart = strings.Trim(modelPart, "'\"")
		return strings.Contains(strings.ToLower(device.Model), strings.ToLower(modelPart))
	default:
		return false
	}
}

// inferYearFromModel extracts year from model name (e.g., "QN90B" -> 2022)
func inferYearFromModel(model string) int {
	model = strings.ToLower(model)

	// Samsung pattern: QN90B (2022), Q80T (2020)
	samsungYears := map[string]int{
		"qn90": 2022, "qn85": 2021, "qn80": 2022, "qn70": 2022,
		"q90": 2019, "q80": 2020, "q70": 2019, "q60": 2020,
		"ls03": 2022, "au80": 2021, "ru71": 2019, "tu80": 2020,
	}

	// LG pattern
	lgYears := map[string]int{
		"c2": 2022, "c1": 2021, "c3": 2023, "g2": 2022, "g1": 2021,
		"a1": 2021, "b1": 2021, "a2": 2022, "u1": 2021,
	}

	// Sony pattern
	sonyYears := map[string]int{
		"a95k": 2022, "a90k": 2022, "a80k": 2022, "x95k": 2022,
		"a9g": 2019, "z9g": 2019, "x900h": 2020, "x90j": 2021,
		"a80j": 2021, "x90k": 2022,
	}

	for prefix, year := range samsungYears {
		if strings.Contains(model, prefix) {
			return year
		}
	}
	for prefix, year := range lgYears {
		if strings.Contains(model, prefix) {
			return year
		}
	}
	for prefix, year := range sonyYears {
		if strings.Contains(model, prefix) {
			return year
		}
	}

	return 0
}

// Helper functions
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

// GetVendorWorkaround returns a workaround for a vendor-specific limitation
func GetVendorWorkaround(manufacturer, limitation string) string {
	rules := GetVendorRules(manufacturer)
	if rules == nil {
		return ""
	}

	for _, rule := range rules {
		switch {
		case limitation == "dts" && rule.NoDTS:
			return rule.Workaround
		case limitation == "dolby_vision" && rule.NoDolbyVision:
			return rule.Workaround
		}
	}
	return ""
}

// GetKnownLimitations returns a list of known limitations for a manufacturer
func GetKnownLimitations(manufacturer string) []string {
	rules := GetVendorRules(manufacturer)
	if rules == nil {
		return nil
	}

	var limitations []string
	for _, rule := range rules {
		if rule.NoDTS {
			limitations = append(limitations, "No DTS passthrough")
		}
		if rule.NoDolbyVision {
			limitations = append(limitations, "No Dolby Vision support")
		}
		if rule.NoHDR10Plus {
			limitations = append(limitations, "No HDR10+ support")
		}
	}
	return limitations
}