// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package android

import (
	"strings"
)

// Android video codec MIME types to Tenkile codec names
var androidVideoCodecs = map[string]string{
	// H.264/AVC
	"video/avc":        "h264",
	"video/h264":       "h264",
	"avc":              "h264",
	"avc1":             "h264",
	"h264":             "h264",
	"h264/mp4":        "h264",

	// H.265/HEVC
	"video/hevc":      "hevc",
	"video/h265":       "hevc",
	"hevc":             "hevc",
	"hevc/mp4":        "hevc",

	// VP8/VP9
	"video/x-vnd.on2.vp8": "vp8",
	"vp8":                 "vp8",
	"video/x-vnd.on2.vp9": "vp9",
	"vp9":                 "vp9",

	// AV1
	"video/av01":       "av1",
	"av01":              "av1",
	"av1":               "av1",
	"av1c":              "av1",

	// MPEG-2
	"video/mpeg2":      "mpeg2",
	"video/mpeg2video": "mpeg2",
	"mpeg2":             "mpeg2",

	// MPEG-4
	"video/mp4v-es":    "mpeg4",
	"video/xvid":       "mpeg4",
	"mpeg4":             "mpeg4",
	"xvid":              "mpeg4",

	// VP6
	"video/vp6":        "vp6",
	"vp6":               "vp6",

	// VC-1
	"video/vc1":        "vc1",
	"vc1":               "vc1",

	// Theora
	"video/theora":     "theora",
	"theora":            "theora",

	// VP9 profiles
	"vp9.0":            "vp9",
	"vp9.1":            "vp9",
	"vp9.2":            "vp9",

	// AV1 profiles
	"av1.0":            "av1",
	"av1.1":            "av1",
	"av1.2":            "av1",

	// Alternative names
	"omx.google.h264.encoder":   "h264",
	"omx.google.hevc.encoder":  "hevc",
	"c2.android.avc.encoder":   "h264",
	"c2.android.hevc.encoder":  "hevc",
	"c2.android.av1.encoder":   "av1",
}

// Android audio codec MIME types to Tenkile codec names
var androidAudioCodecs = map[string]string{
	// AAC
	"audio/mp4a-latm":  "aac",
	"audio/mp4a":       "aac",
	"audio/aac":        "aac",
	"aac":               "aac",
	"aac-lc":           "aac",
	"mp4a-latm":        "aac",

	// AAC HE v1/v2
	"audio/he-aacv1":   "aac_he",
	"audio/he-aacv2":   "aac_he_v2",
	"aac_he":           "aac_he",
	"aac_he_v2":        "aac_he_v2",

	// MP3
	"audio/mpeg":       "mp3",
	"audio/mp3":        "mp3",
	"mp3":               "mp3",

	// FLAC
	"audio/flac":       "flac",
	"flac":              "flac",

	// ALAC
	"audio/alac":       "alac",
	"alac":              "alac",

	// Opus
	"audio/opus":       "opus",
	"opus":              "opus",

	// Dolby Digital (AC-3)
	"audio/ac3":        "ac3",
	"ac3":               "ac3",
	"audio/ac3.0.1":     "ac3",

	// Dolby Digital Plus (E-AC-3)
	"audio/eac3":       "eac3",
	"eac3":              "eac3",
	"audio/eac3.0.1":    "eac3",

	// DTS
	"audio/dts":        "dts",
	"audio/vnd.dts":    "dts",
	"dts":               "dts",
	"audio/dtshd":       "dts",

	// Dolby TrueHD
	"audio/truehd":     "truehd",
	"truehd":            "truehd",
	"audio/mlp":         "truehd",

	// Vorbis
	"audio/vorbis":     "vorbis",
	"vorbis":            "vorbis",

	// G.711
	"audio/g711-alaw":  "pcm_alaw",
	"audio/g711-mlaw":  "pcm_mlaw",
	"pcm_alaw":         "pcm_alaw",
	"pcm_mlaw":         "pcm_mlaw",

	// AMR
	"audio/amr-wb":     "amr_wb",
	"audio/amr-nb":     "amr_nb",
	"amr_wb":           "amr_wb",
	"amr_nb":           "amr_nb",

	// WAV
	"audio/wav":        "wav",
	"wav":               "wav",

	// Alternative names
	"omx.google.aac.encoder":   "aac",
	"c2.android.aac.encoder":  "aac",
	"omx.google.mp3.encoder":  "mp3",
}

// HDR type mapping from Android to Tenkile
var hdrTypeMapping = map[string]struct {
	SupportsHDR         bool
	SupportsDolbyVision bool
	Supports10Bit      bool
}{
	"hdr10":         {SupportsHDR: true, SupportsDolbyVision: false, Supports10Bit: false},
	"hdr10plus":     {SupportsHDR: true, SupportsDolbyVision: false, Supports10Bit: true},
	"dolby-vision":  {SupportsHDR: true, SupportsDolbyVision: true, Supports10Bit: false},
	"dv":            {SupportsHDR: true, SupportsDolbyVision: true, Supports10Bit: false},
	"dv-hevc":       {SupportsHDR: true, SupportsDolbyVision: true, Supports10Bit: false},
	"dv-avc":        {SupportsHDR: true, SupportsDolbyVision: true, Supports10Bit: false},
	"hlg":           {SupportsHDR: true, SupportsDolbyVision: false, Supports10Bit: false},
	"hdr10plus-hevc": {SupportsHDR: true, SupportsDolbyVision: false, Supports10Bit: true},
}

// Container format mapping
var containerMapping = map[string]string{
	// MP4 family
	"video/mp4":       "mp4",
	"video/x-m4v":      "mp4",
	"video/mp4v-es":    "mp4",
	"mp4":               "mp4",
	"m4v":               "mp4",
	"m4a":               "m4a", // audio only

	// MKV/WebM
	"video/x-matroska": "mkv",
	"video/matroska":   "mkv",
	"mkv":               "mkv",
	"x-matroska":        "mkv",
	"matroska":          "mkv",
	"webm":              "webm",
	"video/webm":        "webm",

	// MPEG-TS
	"video/mp2t":       "ts",
	"video/vnd.dlna.mpeg-tts": "ts",
	"video/ts":          "ts",
	"ts":                 "ts",
	"m2ts":               "m2ts",

	// AVI
	"video/avi":        "avi",
	"video/x-msvideo":   "avi",
	"avi":               "avi",

	// MOV
	"video/quicktime":  "mov",
	"mov":               "mov",

	// Flash
	"video/x-flv":      "flv",
	"flv":               "flv",

	// 3GP
	"video/3gpp":       "3gp",
	"3gp":               "3gp",
	"3gpp2":             "3gpp2",
}

// DRM system mapping from Android to standard names
var drmSystemMapping = map[string]string{
	"widevine":     "widevine",
	"playready":    "playready",
	"clearkey":     "clearkey",
	"fairplay":     "fairplay",
	"android-drm":  "widevine", // Generic Android DRM often implies Widevine
}

// MapAndroidCodec maps an Android codec name to a Tenkile codec name.
// isVideo indicates whether this is a video codec (true) or audio codec (false).
func MapAndroidCodec(codec string, isVideo bool) string {
	codec = strings.ToLower(codec)

	// First try exact match with MIME type
	if isVideo {
		if mapped, ok := androidVideoCodecs[codec]; ok {
			return mapped
		}
	} else {
		if mapped, ok := androidAudioCodecs[codec]; ok {
			return mapped
		}
	}

	// Try extracting codec name from MIME type
	// e.g., "OMX.qcom.video.encoder.avc" -> try "avc"
	parts := strings.Split(codec, ".")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if isVideo {
			if mapped, ok := androidVideoCodecs[last]; ok {
				return mapped
			}
		} else {
			if mapped, ok := androidAudioCodecs[last]; ok {
				return mapped
			}
		}
	}

	// Try second to last part for encoder/decoder names
	if len(parts) > 1 {
		second := parts[len(parts)-2]
		switch second {
		case "avc", "avc1":
			return "h264"
		case "hevc", "h265":
			return "hevc"
		case "av1", "av01":
			return "av1"
		case "vp8", "vp9":
			return second
		case "aac", "mp4a":
			return "aac"
		case "ac3", "eac3":
			return second
		}
	}

	// For video codecs, try codec family extraction
	if isVideo {
		switch {
		case strings.Contains(codec, "avc") || strings.Contains(codec, "h264"):
			return "h264"
		case strings.Contains(codec, "hevc") || strings.Contains(codec, "h265"):
			return "hevc"
		case strings.Contains(codec, "av01") || strings.Contains(codec, "av1"):
			return "av1"
		case strings.Contains(codec, "vp9"):
			return "vp9"
		case strings.Contains(codec, "vp8"):
			return "vp8"
		case strings.Contains(codec, "mpeg4") || strings.Contains(codec, "xvid"):
			return "mpeg4"
		case strings.Contains(codec, "mpeg2") || strings.Contains(codec, "mp2"):
			return "mpeg2"
		case strings.Contains(codec, "vc1"):
			return "vc1"
		}
	}

	return ""
}

// MapAndroidContainer maps an Android container MIME type to Tenkile format name
func MapAndroidContainer(container string) string {
	container = strings.ToLower(container)

	if mapped, ok := containerMapping[container]; ok {
		return mapped
	}

	// Try extracting format name
	parts := strings.Split(container, "/")
	if len(parts) > 1 {
		format := parts[len(parts)-1]
		if mapped, ok := containerMapping[format]; ok {
			return mapped
		}
	}

	// Try common formats directly
	switch container {
	case "mkv", "matroska":
		return "mkv"
	case "mp4", "m4v":
		return "mp4"
	case "webm":
		return "webm"
	case "ts", "m2ts":
		return "ts"
	case "avi":
		return "avi"
	case "mov":
		return "mov"
	case "flv":
		return "flv"
	}

	return ""
}

// MapAndroidDRM maps an Android DRM system name to standard name
func MapAndroidDRM(drm string) string {
	drm = strings.ToLower(drm)

	if mapped, ok := drmSystemMapping[drm]; ok {
		return mapped
	}

	return drm
}

// HDRInfo contains parsed HDR capability information
type HDRInfo struct {
	SupportsHDR         bool
	SupportsDolbyVision bool
	Supports10Bit       bool
	SupportsHLG         bool
}

// ParseHDRType parses an HDR type string and returns capability info
func ParseHDRType(hdrType string) HDRInfo {
	hdrType = strings.ToLower(strings.TrimSpace(hdrType))

	if info, ok := hdrTypeMapping[hdrType]; ok {
		return HDRInfo{
			SupportsHDR:         info.SupportsHDR,
			SupportsDolbyVision: info.SupportsDolbyVision,
			Supports10Bit:       info.Supports10Bit,
			SupportsHLG:         hdrType == "hlg",
		}
	}

	// Try matching prefix
	switch {
	case strings.HasPrefix(hdrType, "hdr10"):
		return HDRInfo{SupportsHDR: true, Supports10Bit: hdrType == "hdr10plus"}
	case strings.HasPrefix(hdrType, "dolby") || strings.HasPrefix(hdrType, "dv"):
		return HDRInfo{SupportsHDR: true, SupportsDolbyVision: true}
	case hdrType == "hlg":
		return HDRInfo{SupportsHDR: true, SupportsHLG: true}
	}

	return HDRInfo{}
}
