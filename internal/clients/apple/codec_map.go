// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package apple

import (
	"strconv"
	"strings"
)

// Apple video codec constants to Tenkile codec names
var appleVideoCodecs = map[string]string{
	// H.264/AVC
	"avc1":        "h264",
	"avc1.":       "h264",
	"hvc1":        "hevc",
	"hev1":        "hevc",
	"hev1.":       "hevc",
	"hvc1.":       "hevc",
	"av01":        "av1",
	"av01.":       "av1",

	// HEVC profiles (from VideoToolbox)
	"hvc1.1.6.L120.90":  "hevc", // Main
	"hvc1.1.6.L120.B0":  "hevc", // Main 10
	"hev1.1.6.L120.90":  "hevc", // Main (alternate)
	"hev1.1.6.L120.B0":  "hevc", // Main 10 (alternate)

	// ProRes (macOS)
	"apch":  "prores",
	"apcn":  "prores",
	"apcs":  "prores",
	"apco":  "prores",
	"ap4h":  "prores",
	"prores": "prores",

	// VP9/VP8 (some Apple devices)
	"vp9":  "vp9",
	"vp8":  "vp8",

	// MPEG-2 (rare on Apple)
	"mpg2": "mpeg2",
	"mpv2": "mpeg2",
}

// Apple audio codec constants to Tenkile codec names
var appleAudioCodecs = map[string]string{
	// AAC family
	"aac":      "aac",
	"aac-lc":   "aac",
	"aac-he":   "aac_he",
	"aac-he-v2": "aac_he_v2",
	"aacELD":   "aac",
	"aaceld":   "aac",

	// Apple Lossless
	"alac": "alac",

	// Dolby Digital
	"ac-3":  "ac3",
	"ac3":   "ac3",
	"ec-3":  "eac3",
	"eac3":  "eac3",

	// DTS (on some devices)
	"dts":   "dts",
	"dtshd": "dts",

	// Dolby TrueHD
	"truehd": "truehd",
	"mlpa":   "truehd",

	// MP3
	"mp3":   "mp3",
	"mpeg":  "mp3",

	// FLAC
	"flac":  "flac",

	// Opus (via AudioToolbox)
	"opus":  "opus",

	// Linear PCM
	"lpcm":  "pcm",
	"in24":  "pcm",
	"in32":  "pcm",
	"fl32":  "pcm",
	"fl64":  "pcm",
}

// HDR type mapping for Apple
var appleHDRMapping = map[string]struct {
	SupportsHDR         bool
	SupportsDolbyVision bool
	Supports10Bit      bool
}{
	"hdr10":         {SupportsHDR: true},
	"hdr10plus":     {SupportsHDR: true, Supports10Bit: true},
	"dolby-vision":  {SupportsHDR: true, SupportsDolbyVision: true},
	"dv":            {SupportsHDR: true, SupportsDolbyVision: true},
	"hlg":           {SupportsHDR: true},
}

// Container format mapping
var appleContainerMapping = map[string]string{
	"mov":  "mov",
	"mp4":  "mp4",
	"m4v":  "m4v",
	"m4a":  "m4a",
	"mkv":  "mkv",
	"webm": "webm",
	"ts":   "ts",
	"m2ts": "m2ts",
}

// MapAppleCodec maps an Apple codec name to a Tenkile codec name
func MapAppleCodec(codec string, isVideo bool) string {
	codec = strings.ToLower(codec)

	// Try exact match first
	if isVideo {
		if mapped, ok := appleVideoCodecs[codec]; ok {
			return mapped
		}
		// Try prefix match
		for key, val := range appleVideoCodecs {
			if strings.HasPrefix(codec, key) {
				return val
			}
		}
	} else {
		if mapped, ok := appleAudioCodecs[codec]; ok {
			return mapped
		}
	}

	// Handle codec with profile (e.g., "avc1.64001f")
	if strings.Contains(codec, ".") {
		parts := strings.Split(codec, ".")
		if len(parts) > 0 {
			base := parts[0]
			if isVideo {
				if mapped, ok := appleVideoCodecs[base]; ok {
					return mapped
				}
			} else {
				if mapped, ok := appleAudioCodecs[base]; ok {
					return mapped
				}
			}
		}
	}

	// Fallback: try common codec name patterns
	switch {
	case strings.Contains(codec, "avc") || strings.Contains(codec, "h264"):
		return "h264"
	case strings.Contains(codec, "hevc") || strings.Contains(codec, "h265"):
		return "hevc"
	case strings.Contains(codec, "av01") || strings.Contains(codec, "av1"):
		return "av1"
	case strings.Contains(codec, "aac"):
		return "aac"
	case strings.Contains(codec, "alac"):
		return "alac"
	case strings.Contains(codec, "ac3") || strings.Contains(codec, "ec3"):
		return "ac3"
	case strings.Contains(codec, "eac3"):
		return "eac3"
	case strings.Contains(codec, "flac"):
		return "flac"
	case strings.Contains(codec, "opus"):
		return "opus"
	case strings.Contains(codec, "mp3") || strings.Contains(codec, "mpeg"):
		return "mp3"
	case strings.Contains(codec, "dts"):
		return "dts"
	case strings.Contains(codec, "truehd"):
		return "truehd"
	case strings.Contains(codec, "prores"):
		return "prores"
	}

	return ""
}

// MapAppleContainer maps an Apple container format to Tenkile format
func MapAppleContainer(container string) string {
	container = strings.ToLower(container)

	if mapped, ok := appleContainerMapping[container]; ok {
		return mapped
	}

	return container
}

// ParseAppleHDRType parses an Apple HDR type string
func ParseAppleHDRType(hdrType string) (supportsHDR, supportsDV, supports10Bit bool) {
	hdrType = strings.ToLower(strings.TrimSpace(hdrType))

	if info, ok := appleHDRMapping[hdrType]; ok {
		return info.SupportsHDR, info.SupportsDolbyVision, info.Supports10Bit
	}

	// Try individual types
	if strings.Contains(hdrType, "hdr10") {
		supportsHDR = true
	}
	if strings.Contains(hdrType, "dolby") || strings.Contains(hdrType, "dv") {
		supportsDV = true
		supportsHDR = true
	}
	if strings.Contains(hdrType, "hlg") {
		supportsHDR = true
	}
	if strings.Contains(hdrType, "10") || strings.Contains(hdrType, "10bit") {
		supports10Bit = true
	}

	return
}

// parseCodecList parses a comma-separated list of Apple codec names
func parseCodecList(input string, isVideo bool) []string {
	var codecs []string
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		mapped := MapAppleCodec(part, isVideo)
		if mapped != "" && !contains(codecs, mapped) {
			codecs = append(codecs, mapped)
		} else if mapped == "" {
			// Keep original if not mapped
			if !contains(codecs, part) {
				codecs = append(codecs, part)
			}
		}
	}
	return codecs
}

// parseContainerList parses a comma-separated list of container formats
func parseContainerList(input string) []string {
	var containers []string
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		mapped := MapAppleContainer(part)
		if mapped != "" && !contains(containers, mapped) {
			containers = append(containers, mapped)
		} else if mapped == "" {
			if !contains(containers, part) {
				containers = append(containers, part)
			}
		}
	}
	return containers
}

// parseInt parses a string to int
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	i, _ := strconv.Atoi(s)
	return i
}

// parseInt64 parses a string to int64
func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return true
		}
	}
	return false
}
