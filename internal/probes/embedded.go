// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

//go:embed embedded/*.json
var embeddedFS embed.FS

// EmbeddedDeviceBundle represents a bundle of embedded device profiles
type EmbeddedDeviceBundle struct {
	Devices    []*CuratedDevice `json:"devices"`
	Metadata   BundleMetadata   `json:"_metadata"`
}

// BundleMetadata contains information about the device bundle
type BundleMetadata struct {
	Platform     string `json:"platform"`
	Manufacturer string `json:"manufacturer"`
	Version      string `json:"version"`
	LastUpdated  string `json:"last_updated"`
	Source       string `json:"source"`
	Description  string `json:"description"`
	DeviceCount  int    `json:"device_count"`
}

// EmbeddedLoader handles loading devices from embedded data
type EmbeddedLoader struct {
	mu        sync.RWMutex
	platforms map[string]*EmbeddedDeviceBundle
	loadedAt  time.Time
	version   string
}

// NewEmbeddedLoader creates a new embedded loader
func NewEmbeddedLoader() *EmbeddedLoader {
	return &EmbeddedLoader{
		platforms: make(map[string]*EmbeddedDeviceBundle),
	}
}

// LoadAll loads all embedded device profiles
func (el *EmbeddedLoader) LoadAll() error {
	el.mu.Lock()
	defer el.mu.Unlock()

	// Load each platform file - ALL 12 platforms per DEVICE_DATABASE.md spec
	platforms := []string{
		"samsung_tizen",
		"lg_webos",
		"roku",
		"android_tv",
		"amazon_fire_tv",     // Added: Fire TV is its own platform (not Android TV)
		"apple_tvos",         // Added: tvOS devices
		"philips_android_tv", // Added: Philips Android TVs
		"sony_android_tv",    // Added: Sony Android TVs (separate from generic android_tv)
		"hisense_smart_tv",   // Added: Hisense Vidaa + Android TVs
		"xiaomi_mi_tv",       // Added: Xiaomi Mi TV ecosystem
		"tablets_smartphones", // Added: Mobile devices
		"generic_tv_boxes",   // Added: Generic Android boxes
	}

	for _, platform := range platforms {
		if err := el.loadPlatform(platform); err != nil {
			slog.Warn("Failed to load embedded devices for platform",
				"platform", platform,
				"error", err)
		}
	}

	el.loadedAt = time.Now()
	el.version = "1.1.0"
	slog.Info("Loaded embedded device bundles",
		"platforms", len(el.platforms),
		"loaded_at", el.loadedAt)

	return nil
}

// loadPlatform loads devices for a specific platform
func (el *EmbeddedLoader) loadPlatform(platform string) error {
	filename := fmt.Sprintf("embedded/%s.json", platform)

	data, err := embeddedFS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read embedded file: %w", err)
	}

	var bundle EmbeddedDeviceBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("failed to parse embedded JSON: %w", err)
	}

	// Process metadata
	if bundle.Metadata.Platform == "" {
		bundle.Metadata.Platform = platform
	}
	bundle.Metadata.DeviceCount = len(bundle.Devices)

	el.platforms[platform] = &bundle
	slog.Debug("Loaded embedded devices",
		"platform", platform,
		"count", len(bundle.Devices))

	return nil
}

// GetPlatforms returns all loaded platforms
func (el *EmbeddedLoader) GetPlatforms() []string {
	el.mu.RLock()
	defer el.mu.RUnlock()

	platforms := make([]string, 0, len(el.platforms))
	for p := range el.platforms {
		platforms = append(platforms, p)
	}
	return platforms
}

// GetDevices returns all devices for a platform
func (el *EmbeddedLoader) GetDevices(platform string) []*CuratedDevice {
	el.mu.RLock()
	defer el.mu.RUnlock()

	if bundle, ok := el.platforms[platform]; ok {
		return bundle.Devices
	}
	return nil
}

// GetDevice returns a specific device by ID from embedded data
func (el *EmbeddedLoader) GetDevice(platform, deviceID string) *CuratedDevice {
	el.mu.RLock()
	defer el.mu.RUnlock()

	if bundle, ok := el.platforms[platform]; ok {
		for _, device := range bundle.Devices {
			if device.ID == deviceID {
				return device
			}
		}
	}
	return nil
}

// GetMetadata returns metadata for a platform
func (el *EmbeddedLoader) GetMetadata(platform string) *BundleMetadata {
	el.mu.RLock()
	defer el.mu.RUnlock()

	if bundle, ok := el.platforms[platform]; ok {
		return &bundle.Metadata
	}
	return nil
}

// GetAllDevices returns all devices from all platforms
func (el *EmbeddedLoader) GetAllDevices() []*CuratedDevice {
	el.mu.RLock()
	defer el.mu.RUnlock()

	var all []*CuratedDevice
	for _, bundle := range el.platforms {
		all = append(all, bundle.Devices...)
	}
	return all
}

// GetTotalCount returns the total number of embedded devices
func (el *EmbeddedLoader) GetTotalCount() int {
	el.mu.RLock()
	defer el.mu.RUnlock()

	total := 0
	for _, bundle := range el.platforms {
		total += len(bundle.Devices)
	}
	return total
}

// LoadIntoCuratedDB loads all embedded devices into a CuratedDatabase
func (el *EmbeddedLoader) LoadIntoCuratedDB(db *CuratedDatabase) error {
	el.mu.RLock()
	defer el.mu.RUnlock()

	for platform, bundle := range el.platforms {
		for _, device := range bundle.Devices {
			// Ensure device has platform set
			if device.Platform == "" {
				device.Platform = platform
			}
			if err := db.addDevice(device); err != nil {
				slog.Warn("Failed to add embedded device",
					"device_id", device.ID,
					"platform", platform,
					"error", err)
			}
		}
	}

	return nil
}

// GetBundleInfo returns information about a specific bundle
func (el *EmbeddedLoader) GetBundleInfo(platform string) map[string]interface{} {
	el.mu.RLock()
	defer el.mu.RUnlock()

	info := make(map[string]interface{})

	if bundle, ok := el.platforms[platform]; ok {
		info["platform"] = bundle.Metadata.Platform
		info["manufacturer"] = bundle.Metadata.Manufacturer
		info["version"] = bundle.Metadata.Version
		info["last_updated"] = bundle.Metadata.LastUpdated
		info["source"] = bundle.Metadata.Source
		info["description"] = bundle.Metadata.Description
		info["device_count"] = len(bundle.Devices)

		// Count by source
		sourceCount := make(map[string]int)
		verifiedCount := 0
		for _, d := range bundle.Devices {
			sourceCount[d.Source]++
			if d.Verified {
				verifiedCount++
			}
		}
		info["source_distribution"] = sourceCount
		info["verified_count"] = verifiedCount
	}

	return info
}

// GetAllBundlesInfo returns information about all bundles
func (el *EmbeddedLoader) GetAllBundlesInfo() map[string]map[string]interface{} {
	el.mu.RLock()
	defer el.mu.RUnlock()

	result := make(map[string]map[string]interface{})
	for platform := range el.platforms {
		result[platform] = el.GetBundleInfo(platform)
	}
	return result
}

// IsLoaded returns whether the embedded data has been loaded
func (el *EmbeddedLoader) IsLoaded() bool {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return !el.loadedAt.IsZero()
}

// GetVersion returns the embedded data version
func (el *EmbeddedLoader) GetVersion() string {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.version
}

// GetLoadTime returns when the data was loaded
func (el *EmbeddedLoader) GetLoadTime() time.Time {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.loadedAt
}

// Global embedded loader instance
var globalEmbeddedLoader = NewEmbeddedLoader()

// InitEmbeddedLoader initializes the global embedded loader
func InitEmbeddedLoader() error {
	return globalEmbeddedLoader.LoadAll()
}

// GetEmbeddedLoader returns the global embedded loader
func GetEmbeddedLoader() *EmbeddedLoader {
	return globalEmbeddedLoader
}
