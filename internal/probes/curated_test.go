// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"testing"
)

// TestCuratedDevicesMatchSpec verifies that curated devices match DEVICE_DATABASE.md specifications
func TestCuratedDevicesMatchSpec(t *testing.T) {
	// Initialize embedded loader if not already done
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	tests := []struct {
		name       string
		deviceID   string
		codec      string
		shouldHave bool
		reason     string
	}{
		// Samsung tests (DEVICE_DATABASE.md Section 4.1)
		{"Samsung 2022+ has AV1", "samsung-qn90b-2022", "av1", true, "Samsung Tizen 2022+ supports AV1 (Neo QLED)"},
		{"Samsung 2022 has AV1 (Q80C)", "samsung-qa65q80c-2023", "av1", true, "Samsung 2023 has AV1"},
		{"Samsung 2022 8K has AV1", "samsung-qn900c-2023", "av1", true, "Samsung 8K 2023 has AV1"},
		{"Samsung 2020 has no AV1", "samsung-q80t-2020", "av1", false, "Samsung 2020 (pre-Neo QLED) has no AV1"},
		{"Samsung 2019 has no AV1", "samsung-q70r-2019", "av1", false, "Samsung 2019 has no AV1"},
		{"Samsung never has DV", "samsung-qn90b-2022", "dolby_vision", false, "Samsung never supports Dolby Vision"},
		{"Samsung never has DTS", "samsung-qn90b-2022", "dts", false, "Samsung Tizen never supports DTS"},
		{"Samsung has SSA/ASS", "samsung-qn90b-2022", "ssa", true, "Samsung supports SSA/ASS subtitles"},

		// LG tests (DEVICE_DATABASE.md Section 4.2)
		{"LG always has DV", "lg-c2-2022", "dolby_vision", true, "LG always supports Dolby Vision"},
		{"LG 2022+ has AV1", "lg-c2-2022", "av1", true, "LG 2022+ (α7 Gen 5) has AV1"},
		// Note: LG never adopted HDR10+ - this is informational, not a capability flag
		{"LG has SSA/ASS", "lg-c2-2022", "ssa", true, "LG supports SSA/ASS subtitles"},

		// Amazon Fire TV tests
		{"Fire TV Cube 3rd gen has AV1", "amazon-fire-tv-cube-3rd", "av1", true, "Fire TV Cube 3rd gen supports AV1"},
		{"Fire TV has no DTS", "amazon-fire-tv-cube-3rd", "dts", false, "Fire TV never supports DTS"},
		{"Fire TV has DV", "amazon-fire-tv-cube-3rd", "dolby_vision", true, "Fire TV supports DV Profile 8"},
		{"Fire TV 4K Stick has no AV1 (2017)", "amazon-fire-tv-4k", "av1", false, "Original Fire TV Stick 4K has no AV1"},
		{"Fire TV 4K Max has AV1", "amazon-fire-tv-4k-max", "av1", true, "Fire TV Stick 4K Max supports AV1"},

		// Android TV tests
		{"Chromecast with Google TV has AV1", "chromecast-with-google-tv", "av1", true, "MT8695 supports AV1"},
		{"NVIDIA Shield has AV1", "nvidia-shield-pro-2019", "av1", true, "Tegra X1+ supports AV1"},
		{"NVIDIA Shield has DTS", "nvidia-shield-pro-2019", "dts", true, "Shield supports DTS"},
		{"Mi Box S has no AV1", "xiaomi-mi-box-s", "av1", false, "Mi Box S (S905X3) has no AV1"},
		{"Mi Box S has no TrueHD", "xiaomi-mi-box-s", "truehd", false, "Mi Box S doesn't support TrueHD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get device from embedded loader
			var device *CuratedDevice
			var found bool

			// Search through all platforms
			for _, p := range loader.GetPlatforms() {
				devices := loader.GetDevices(p)
				for _, d := range devices {
					if d.ID == tt.deviceID {
						device = d
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				t.Skipf("Device %s not found in embedded data", tt.deviceID)
				return
			}

			// Apply vendor rules before checking
			ApplyVendorRules(device)

			hasCodec := false
			switch tt.codec {
			case "av1":
				hasCodec = containsString(device.Capabilities.VideoCodecs, "av1")
			case "dolby_vision":
				hasCodec = device.Capabilities.SupportsDolbyVision
			case "dts":
				hasCodec = device.Capabilities.SupportsDTS
			case "truehd":
				hasCodec = containsString(device.Capabilities.AudioCodecs, "truehd")
			case "ssa":
				hasCodec = containsString(device.Capabilities.SubtitleFormats, "ssa")
			case "hdr10plus":
				// HDR10+ is implied by HDR support but vendor-specific
				hasCodec = device.Capabilities.SupportsHDR
			}

			if hasCodec != tt.shouldHave {
				t.Errorf("Device %s: %s - got %v, want %v (%s)",
					tt.deviceID, tt.codec, hasCodec, tt.shouldHave, tt.reason)
			}
		})
	}
}

// TestSamsungNoDV verifies Samsung never supports Dolby Vision
func TestSamsungNoDV(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	samsungDevices := loader.GetDevices("samsung_tizen")
	for _, device := range samsungDevices {
		ApplyVendorRules(device)
		if device.Capabilities.SupportsDolbyVision {
			t.Errorf("Samsung device %s has Dolby Vision support - Samsung should never support DV", device.ID)
		}
	}
}

// TestSamsungNoDTS verifies Samsung never supports DTS
func TestSamsungNoDTS(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	samsungDevices := loader.GetDevices("samsung_tizen")
	for _, device := range samsungDevices {
		ApplyVendorRules(device)
		if device.Capabilities.SupportsDTS {
			t.Errorf("Samsung device %s has DTS support - Samsung should never support DTS", device.ID)
		}
		if containsString(device.Capabilities.AudioCodecs, "dts") {
			t.Errorf("Samsung device %s has DTS in audio codecs", device.ID)
		}
	}
}

// TestLGalwaysDV verifies LG always supports Dolby Vision
func TestLGalwaysDV(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	lgDevices := loader.GetDevices("lg_webos")
	for _, device := range lgDevices {
		ApplyVendorRules(device)
		if !device.Capabilities.SupportsDolbyVision {
			t.Errorf("LG device %s lacks Dolby Vision - LG should always support DV", device.ID)
		}
	}
}

// TestFireTVNoDTS verifies Fire TV never supports DTS
func TestFireTVNoDTS(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	fireDevices := loader.GetDevices("amazon_fire_tv")
	for _, device := range fireDevices {
		ApplyVendorRules(device)
		if device.Capabilities.SupportsDTS {
			t.Errorf("Fire TV device %s has DTS - Fire TV should never support DTS", device.ID)
		}
	}
}

// TestSoCInference tests SoC-based capability inference
func TestSoCInference(t *testing.T) {
	tests := []struct {
		name     string
		soc      string
		year     int
		wantAV1  bool
		wantDV   bool
		wantDTS  bool
	}{
		{"Exynos M7 (2022+)", "Exynos M7", 2022, true, false, false},
		{"Exynos M5 (2020)", "Exynos M5", 2020, false, false, false},
		{"α9 Gen 6 (2022)", "α9 Gen 6", 2022, true, true, true},
		{"Tegra X1+ (2019)", "Tegra X1+", 2019, true, true, true},
		{"Amlogic S905X4", "Amlogic S905X4", 2021, true, true, true}, // Note: some S905X4 boxes support DTS via HDMI
		{"A15 Bionic", "A15 Bionic", 2023, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := InferCapabilitiesFromSoC(tt.soc, tt.year)
			if caps == nil {
				t.Fatalf("InferCapabilitiesFromSoC returned nil for SoC %s", tt.soc)
			}

			hasAV1 := containsString(caps.VideoCodecs, "av1")
			hasDV := caps.SupportsDolbyVision
			hasDTS := caps.SupportsDTS

			if hasAV1 != tt.wantAV1 {
				t.Errorf("SoC %s AV1: got %v, want %v", tt.soc, hasAV1, tt.wantAV1)
			}
			if hasDV != tt.wantDV {
				t.Errorf("SoC %s DV: got %v, want %v", tt.soc, hasDV, tt.wantDV)
			}
			if hasDTS != tt.wantDTS {
				t.Errorf("SoC %s DTS: got %v, want %v", tt.soc, hasDTS, tt.wantDTS)
			}
		})
	}
}

// TestSoCAliases verifies SoC alias resolution works
func TestSoCAliases(t *testing.T) {
	aliases := []string{
		"Exynos M7",
		"exynos_m7",
		"exynos_9",
		"α9 Gen 6",
		"a9gen6",
		"Amlogic S905X4",
		"s905x4",
		"Tegra X1+",
		"tegra_x1_plus",
	}

	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			caps, ok := GetSoCCapabilities(alias)
			if !ok {
				t.Errorf("Failed to resolve SoC alias: %s", alias)
			}
			if len(caps.VideoCodecs) == 0 {
				t.Errorf("SoC %s returned empty codec list", alias)
			}
		})
	}
}

// TestVendorRules verifies vendor rules are applied correctly
func TestVendorRules(t *testing.T) {
	tests := []struct {
		manufacturer string
		limitation   string
		expectFalse  bool
	}{
		{"Samsung", "dts", true},
		{"Samsung", "dolby_vision", true},
		{"LG", "dolby_vision", false},
		{"Amazon", "dts", true},
		{"Apple", "dts", true},
		{"NVIDIA", "dts", false},
	}

	for _, tt := range tests {
		t.Run(tt.manufacturer+"_"+tt.limitation, func(t *testing.T) {
			workaround := GetVendorWorkaround(tt.manufacturer, tt.limitation)
			limitations := GetKnownLimitations(tt.manufacturer)

			hasLimitation := false
			for _, lim := range limitations {
				switch tt.limitation {
				case "dts":
					hasLimitation = hasLimitation || lim == "No DTS passthrough"
				case "dolby_vision":
					hasLimitation = hasLimitation || lim == "No Dolby Vision support"
				}
			}

			if hasLimitation != tt.expectFalse {
				t.Errorf("Vendor %s limitation %s: got %v, want %v",
					tt.manufacturer, tt.limitation, hasLimitation, tt.expectFalse)
			}

			if tt.expectFalse && workaround == "" {
				t.Errorf("Vendor %s should have workaround for %s", tt.manufacturer, tt.limitation)
			}
		})
	}
}

// TestAllPlatformsLoad verifies all 12 platforms load successfully
func TestAllPlatformsLoad(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	expectedPlatforms := map[string]bool{
		"samsung_tizen":       false,
		"lg_webos":            false,
		"roku":                 false,
		"android_tv":          false,
		"amazon_fire_tv":      false,
		"apple_tvos":           false,
		"philips_android_tv":   false,
		"sony_android_tv":      false,
		"hisense_smart_tv":     false,
		"xiaomi_mi_tv":         false,
		"tablets_smartphones":  false,
		"generic_tv_boxes":     false,
	}

	loadedPlatforms := loader.GetPlatforms()
	for _, p := range loadedPlatforms {
		if _, ok := expectedPlatforms[p]; ok {
			expectedPlatforms[p] = true
		}
	}

	missingPlatforms := []string{}
	for platform, loaded := range expectedPlatforms {
		if !loaded {
			missingPlatforms = append(missingPlatforms, platform)
		}
	}

	if len(missingPlatforms) > 0 {
		t.Errorf("Missing platforms: %v", missingPlatforms)
	}

	// Verify total device count
	totalDevices := loader.GetTotalCount()
	if totalDevices < 50 {
		t.Errorf("Expected at least 50 devices across all platforms, got %d", totalDevices)
	}
}

// TestSamsungQN90BAV1 specifically tests the QN90B AV1 fix
func TestSamsungQN90BAV1(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	device := loader.GetDevice("samsung_tizen", "samsung-qn90b-2022")
	if device == nil {
		t.Fatal("Samsung QN90B 2022 not found in embedded data")
	}

	ApplyVendorRules(device)

	hasAV1 := containsString(device.Capabilities.VideoCodecs, "av1")
	if !hasAV1 {
		t.Errorf("Samsung QN90B (2022) should have AV1 support - DEVICE_DATABASE.md Section 4.1 says 2022+ Samsung has AV1")
	}

	// Verify no DV
	if device.Capabilities.SupportsDolbyVision {
		t.Errorf("Samsung QN90B should NOT have Dolby Vision")
	}

	// Verify no DTS
	if device.Capabilities.SupportsDTS {
		t.Errorf("Samsung QN90B should NOT have DTS")
	}
}

// TestAmazonFireTVCube3rdGen verifies Fire TV Cube 3rd gen capabilities
func TestAmazonFireTVCube3rdGen(t *testing.T) {
	loader := GetEmbeddedLoader()
	if loader == nil || !loader.IsLoaded() {
		loader = NewEmbeddedLoader()
		if err := loader.LoadAll(); err != nil {
			t.Fatalf("Failed to load embedded devices: %v", err)
		}
	}

	// First check if it's in the Fire TV platform
	device := loader.GetDevice("amazon_fire_tv", "amazon-fire-tv-cube-3rd")
	if device == nil {
		t.Fatal("Amazon Fire TV Cube 3rd Gen not found")
	}

	ApplyVendorRules(device)

	// Should have AV1 (3rd gen)
	hasAV1 := containsString(device.Capabilities.VideoCodecs, "av1")
	if !hasAV1 {
		t.Errorf("Fire TV Cube 3rd gen should have AV1 support")
	}

	// Should NOT have DTS
	if device.Capabilities.SupportsDTS {
		t.Errorf("Fire TV Cube 3rd gen should NOT have DTS support")
	}

	// Should have Dolby Vision
	if !device.Capabilities.SupportsDolbyVision {
		t.Errorf("Fire TV Cube 3rd gen should have Dolby Vision")
	}
}

// TestDeviceClasses verifies device class inference
func TestDeviceClasses(t *testing.T) {
	tests := []struct {
		soc       string
		year      int
		maxRes    int
		wantClass string
	}{
		{"Exynos M7", 2022, 4320, "A"},
		{"Exynos M7", 2022, 2160, "A"},
		{"α9 Gen 6", 2022, 2160, "A"},
		{"Amlogic S905X4", 2021, 2160, "C"}, // Mid-range class due to year
		{"MediaTek MT8163", 2020, 2160, "C"},
		{"Amlogic S905W", 2018, 2160, "C"}, // Budget but not premium enough for D
	}

	for _, tt := range tests {
		t.Run(tt.soc, func(t *testing.T) {
			gotClass := InferDeviceClass(tt.soc, tt.year, tt.maxRes)
			if gotClass != tt.wantClass {
				t.Errorf("InferDeviceClass(%s, %d, %d): got %s, want %s",
					tt.soc, tt.year, tt.maxRes, gotClass, tt.wantClass)
			}
		})
	}
}
