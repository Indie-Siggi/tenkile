// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"strings"
)

// CodecCapabilities represents codec support for a given SoC
type CodecCapabilities struct {
	VideoCodecs []string `json:"video_codecs"`
	HDR         []string `json:"hdr"`          // "hdr10", "hdr10+", "hlg", "dolby_vision"
	DolbyVision bool     `json:"dolby_vision"`
	DTS         bool     `json:"dts"`
	TrueHD      bool     `json:"truehd"`
	MaxRes      int      `json:"max_res"` // 2160 = 4K, 4320 = 8K
	MaxBitrate  int64    `json:"max_bitrate"`
}

// SoC Capability Table - Maps SoC names to codec capabilities
// Based on DEVICE_DATABASE.md Section 6
var socCapabilityTable = map[string]CodecCapabilities{
	// Samsung Exynos SoCs
	"Exynos M7": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "hdr10+"},
		DolbyVision: false, // Samsung never supports DV
		DTS:         false, // Samsung never supports DTS
		TrueHD:      true,
		MaxRes:      4320,
		MaxBitrate:  200000000,
	},
	"Exynos M6": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "hdr10+"},
		DolbyVision: false,
		DTS:         false,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  125000000,
	},
	"Exynos M5": {
		VideoCodecs: []string{"h264", "hevc", "vp9"},
		HDR:         []string{"hdr10", "hdr10+"},
		DolbyVision: false,
		DTS:         false,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},
	"Exynos M4": {
		VideoCodecs: []string{"h264", "hevc", "vp9"},
		HDR:         []string{"hdr10"},
		DolbyVision: false,
		DTS:         false,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  80000000,
	},

	// LG α (Alpha) SoCs
	"α9 Gen 6": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "hlg", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      4320,
		MaxBitrate:  200000000,
	},
	"α9 Gen 5": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "hlg", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  125000000,
	},
	"α7 Gen 7": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "hlg", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},

	// Amlogic SoCs (generic Android TV boxes)
	"Amlogic S905X4": {
		VideoCodecs: []string{"h264", "hevc", "av1"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      false, // Limited audio
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},
	"Amlogic S905X3": {
		VideoCodecs: []string{"h264", "hevc", "av1"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  80000000,
	},
	"Amlogic S905X2": {
		VideoCodecs: []string{"h264", "hevc"},
		HDR:         []string{"hdr10"},
		DolbyVision: false,
		DTS:         true,
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  60000000,
	},
	"Amlogic S905W": {
		VideoCodecs: []string{"h264", "hevc"},
		HDR:         []string{"hdr10"},
		DolbyVision: false,
		DTS:         true,
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  40000000,
	},

	// NVIDIA Tegra (Shield)
	"Tegra X1+": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  200000000,
	},

	// MediaTek SoCs
	"MediaTek MT8163": {
		VideoCodecs: []string{"h264", "hevc", "vp9"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  60000000,
	},
	"MediaTek MT7921": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},
	"MediaTek MT8695": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  120000000,
	},

	// Apple Silicon
	"A15 Bionic": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "dolby_vision", "hlg"},
		DolbyVision: true,
		DTS:         false, // Apple doesn't support DTS
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},
	"A14 Bionic": {
		VideoCodecs: []string{"h264", "hevc", "vp9"},
		HDR:         []string{"hdr10", "dolby_vision", "hlg"},
		DolbyVision: true,
		DTS:         false,
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},
	"A12 Bionic": {
		VideoCodecs: []string{"h264", "hevc", "vp9"},
		HDR:         []string{"hdr10", "dolby_vision", "hlg"},
		DolbyVision: true,
		DTS:         false,
		TrueHD:      false,
		MaxRes:      2160,
		MaxBitrate:  80000000,
	},

	// Realtek (generic boxes)
	"Realtek RTD1295": {
		VideoCodecs: []string{"h264", "hevc", "vp9"},
		HDR:         []string{"hdr10"},
		DolbyVision: false,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  100000000,
	},
	"Realtek RTD1319": {
		VideoCodecs: []string{"h264", "hevc", "vp9", "av1"},
		HDR:         []string{"hdr10", "dolby_vision"},
		DolbyVision: true,
		DTS:         true,
		TrueHD:      true,
		MaxRes:      2160,
		MaxBitrate:  120000000,
	},
}

// SoC aliases for matching
var socAliases = map[string]string{
	"exynos_m7":        "Exynos M7",
	"exynos_9":         "Exynos M7", // Exynos 9 series is M7-based
	"exynos_m6":        "Exynos M6",
	"exynos_m5":        "Exynos M5",
	"exynos_m4":        "Exynos M4",
	"exynos_9630":      "Exynos M5",
	"exynos_9638":      "Exynos M4",
	"alpha9":           "α9 Gen 6",
	"alpha_9":          "α9 Gen 6",
	"alpha_7":          "α7 Gen 7",
	"a9gen6":           "α9 Gen 6",
	"a9gen5":           "α9 Gen 5",
	"a7gen7":           "α7 Gen 7",
	"s905x4":           "Amlogic S905X4",
	"s905x3":           "Amlogic S905X3",
	"s905x2":           "Amlogic S905X2",
	"s905w":            "Amlogic S905W",
	"amlogic_t965":     "Amlogic S905X4", // T965 is similar generation
	"tegra_x1_plus":    "Tegra X1+",
	"tegra_x1":         "Tegra X1+",
	"mt8163":           "MediaTek MT8163",
	"mt7921":           "MediaTek MT7921",
	"mt8695":           "MediaTek MT8695",
	"mt8581":           "MediaTek MT8695", // TV chip
	"a15":              "A15 Bionic",
	"a14":              "A14 Bionic",
	"a12":              "A12 Bionic",
	"apple_tv_a15":     "A15 Bionic",
	"rtd1295":          "Realtek RTD1295",
	"rtd1319":          "Realtek RTD1319",
	"realtek_rtd1295": "Realtek RTD1295",
	"realtek_rtd1319": "Realtek RTD1319",
}

// GetSoCCapabilities returns codec capabilities for a given SoC name
func GetSoCCapabilities(socName string) (CodecCapabilities, bool) {
	// Direct lookup
	if caps, ok := socCapabilityTable[socName]; ok {
		return caps, true
	}

	// Alias lookup
	normalized := strings.ToLower(strings.ReplaceAll(socName, " ", "_"))
	if alias, ok := socAliases[normalized]; ok {
		if caps, ok := socCapabilityTable[alias]; ok {
			return caps, true
		}
	}

	// Partial match (for SoCs like "Exynos M7 Pro")
	for key, caps := range socCapabilityTable {
		if strings.HasPrefix(strings.ToLower(socName), strings.ToLower(key)) {
			return caps, true
		}
	}

	return CodecCapabilities{}, false
}

// InferCapabilitiesFromSoC infers device capabilities from SoC information
func InferCapabilitiesFromSoC(socName string, year int) *DeviceCapabilities {
	caps, ok := GetSoCCapabilities(socName)
	if !ok {
		return nil
	}

	capabilities := &DeviceCapabilities{
		VideoCodecs:      caps.VideoCodecs,
		MaxWidth:         3840,
		MaxHeight:        caps.MaxRes,
		MaxBitrate:       caps.MaxBitrate,
		SupportsHDR:      len(caps.HDR) > 0,
		SupportsDolbyVision: caps.DolbyVision,
		SupportsDTS:       caps.DTS,
	}

	// Set audio codecs based on SoC
	capabilities.AudioCodecs = []string{"aac", "ac3", "eac3", "opus", "mp3"}
	if caps.TrueHD {
		capabilities.AudioCodecs = append(capabilities.AudioCodecs, "truehd", "flac")
	}
	if caps.DTS {
		capabilities.AudioCodecs = append(capabilities.AudioCodecs, "dts", "dtshd")
	}

	// Infer subtitle formats based on platform
	capabilities.SubtitleFormats = []string{"srt", "vtt"}

	// Infer container formats
	capabilities.ContainerFormats = []string{"mp4", "mkv", "ts"}

	// Year-based AV1 support overrides (Samsung 2022+, LG 2022+, etc.)
	// These are already captured in the SoC table for recent chips

	return capabilities
}

// InferDeviceClass determines device class from SoC and year
func InferDeviceClass(socName string, year int, maxRes int) string {
	_, ok := GetSoCCapabilities(socName)
	if !ok {
		return "D" // Unknown/default
	}

	// Premium devices (Class A)
	if maxRes >= 4320 || (strings.Contains(strings.ToLower(socName), "m7") ||
		strings.Contains(strings.ToLower(socName), "α9 gen 6") ||
		strings.Contains(strings.ToLower(socName), "a15")) {
		return "A"
	}

	// High-end (Class B)
	if maxRes >= 2160 && year >= 2022 {
		return "B"
	}

	// Mid-range (Class C)
	if maxRes >= 2160 {
		return "C"
	}

	// Budget (Class D)
	return "D"
}
