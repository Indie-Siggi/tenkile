// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package codec

import (
	"fmt"
	"strings"
)

// MIMEType represents a MIME type with codec information
type MIMEType struct {
	Type        string   `json:"type"`
	Subtype     string   `json:"subtype"`
	Codec       string   `json:"codec,omitempty"`
	Parameters  []string `json:"parameters,omitempty"`
	IsVideo     bool     `json:"is_video"`
	IsAudio     bool     `json:"is_audio"`
	IsSubtitle  bool     `json:"is_subtitle"`
}

// String returns the full MIME type string
func (m *MIMEType) String() string {
	s := fmt.Sprintf("%s/%s", m.Type, m.Subtype)
	if m.Codec != "" {
		s += fmt.Sprintf("; codecs=%s", m.Codec)
	}
	return s
}

// VideoMIMEs defines MIME types for video codecs
var VideoMIMEs = map[string]string{
	"h264": "video/mp4; codecs=\"avc1\"",
	"hevc": "video/mp4; codecs=\"hvc1\"",
	"vp9":  "video/webm; codecs=\"vp9\"",
	"av1":  "video/webm; codecs=\"av01\"",
	"vp8":  "video/webm; codecs=\"vp8\"",
	"mpeg2": "video/mpeg",
	"mpeg4": "video/mp4; codecs=\"mp4v\"",
	"vc1":   "video/vc1",
}

// AudioMIMEs defines MIME types for audio codecs
var AudioMIMEs = map[string]string{
	"aac":      "audio/mp4; codecs=\"mp4a\"",
	"mp3":      "audio/mpeg",
	"flac":     "audio/flac",
	"alac":     "audio/alac",
	"opus":     "audio/opus",
	"ac3":      "audio/ac3",
	"eac3":     "audio/eac3",
	"dts":      "audio/vnd.dts",
	"truehd":   "audio/vnd.dolby.dd-truehd",
	"aac_low":  "audio/mp4; codecs=\"mp4a.40.2\"",
	"aac_he":   "audio/mp4; codecs=\"mp4a.40.5\"",
	"aac_he_v2": "audio/mp4; codecs=\"mp4a.40.29\"",
}

// SubtitleMIMEs defines MIME types for subtitle formats
var SubtitleMIMEs = map[string]string{
	"vtt":  "text/vtt",
	"ssa":  "text/x-ssa",
	"ass":  "text/x-ssa",
	"srt":  "application/x-subrip",
	"sub":  "video/x-sub",
	"pgs":  "video/x-hdmv-subpicture",
	"dvb":  "video/x-dvb-subtitle",
	"dvd":  "video/x-dvd-subtitle",
}

// ContainerMIMEs defines MIME types for container formats
var ContainerMIMEs = map[string]string{
	"mp4":  "video/mp4",
	"mkv":  "video/x-matroska",
	"ts":   "video/mp2t",
	"webm": "video/webm",
	"mov":  "video/quicktime",
	"avi":  "video/x-msvideo",
	"flv":  "video/x-flv",
	"mpg":  "video/mpeg",
	"mpeg": "video/mpeg",
	"wmv":  "video/x-ms-wmv",
}

// GetVideoMIME returns the MIME type for a video codec
func GetVideoMIME(codec string) string {
	if mime, ok := VideoMIMEs[codec]; ok {
		return mime
	}
	return "video/mp4"
}

// GetAudioMIME returns the MIME type for an audio codec
func GetAudioMIME(codec string) string {
	if mime, ok := AudioMIMEs[codec]; ok {
		return mime
	}
	return "audio/mp4"
}

// GetSubtitleMIME returns the MIME type for a subtitle format
func GetSubtitleMIME(format string) string {
	if mime, ok := SubtitleMIMEs[format]; ok {
		return mime
	}
	return "text/vtt"
}

// GetContainerMIME returns the MIME type for a container format
func GetContainerMIME(container string) string {
	if mime, ok := ContainerMIMEs[container]; ok {
		return mime
	}
	return "application/octet-stream"
}

// ParseMIME parses a MIME type string and extracts codec information
func ParseMIME(mime string) (*MIMEType, error) {
	parts := strings.SplitN(mime, ";", 2)
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("invalid MIME type: empty string")
	}

	typeParts := strings.SplitN(parts[0], "/", 2)
	if len(typeParts) != 2 {
		return nil, fmt.Errorf("invalid MIME type: %s", mime)
	}

	mimeType := &MIMEType{
		Type:    strings.TrimSpace(typeParts[0]),
		Subtype: strings.TrimSpace(typeParts[1]),
	}

	// Determine media type
	switch mimeType.Type {
	case "video":
		mimeType.IsVideo = true
	case "audio":
		mimeType.IsAudio = true
	case "text":
		mimeType.IsSubtitle = true
	}

	// Parse parameters
	if len(parts) > 1 {
		params := strings.Split(parts[1], ",")
		for _, param := range params {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "codecs=") {
				codec := strings.TrimPrefix(param, "codecs=")
				codec = strings.Trim(codec, "\"")
				mimeType.Codec = codec
			} else if param != "" {
				mimeType.Parameters = append(mimeType.Parameters, param)
			}
		}
	}

	return mimeType, nil
}

// GetCodecFromMIME extracts the codec name from a MIME type string
func GetCodecFromMIME(mime string) (string, error) {
	parsed, err := ParseMIME(mime)
	if err != nil {
		return "", err
	}

	if parsed.Codec != "" {
		// Try to map common codec abbreviations
		codecMap := map[string]string{
			"avc1":     "h264",
			"hvc1":     "hevc",
			"vp09":     "vp9",
			"av01":     "av1",
			"vp8":      "vp8",
			"mp4a":     "aac",
			"mp4a.40.2": "aac_low",
			"mp4a.40.5": "aac_he",
			"mp4a.40.29": "aac_he_v2",
			"mp4a.69":   "aac_he",
			"mp4a.6B":   "aac_he_v2",
			"mp3":       "mp3",
			"ac-3":      "ac3",
			"ec-3":      "eac3",
			"dtsc":      "dts",
			"dtsh":      "dts",
			"dtse":      "dts",
			"dtshd":     "dts",
		}

		codec := strings.ToLower(parsed.Codec)
		if mapped, ok := codecMap[codec]; ok {
			return mapped, nil
		}

		// Check if it's a known codec
		if _, ok := codecs[codec]; ok {
			return codec, nil
		}

		// Return as-is if not mapped
		return codec, nil
	}

	return "", fmt.Errorf("no codec found in MIME type: %s", mime)
}

// GetExtensionFromMIME returns the file extension for a MIME type
func GetExtensionFromMIME(mime string) string {
	// Check container MIMEs first
	for ext, containerMime := range ContainerMIMEs {
		if strings.HasPrefix(mime, containerMime) {
			switch ext {
			case "mp4":
				return ".mp4"
			case "mkv":
				return ".mkv"
			case "ts":
				return ".ts"
			case "webm":
				return ".webm"
			case "mov":
				return ".mov"
			case "avi":
				return ".avi"
			case "flv":
				return ".flv"
			case "mpg", "mpeg":
				return ".mpg"
			case "wmv":
				return ".wmv"
			}
		}
	}

	// Check video codecs
	for codec, videoMime := range VideoMIMEs {
		if strings.HasPrefix(mime, videoMime) {
			if codec == "hevc" || codec == "h264" {
				return ".mp4"
			}
			return ".webm"
		}
	}

	// Check audio codecs
	for codec, audioMime := range AudioMIMEs {
		if strings.HasPrefix(mime, audioMime) {
			switch codec {
			case "mp3":
				return ".mp3"
			case "flac":
				return ".flac"
			case "opus":
				return ".opus"
			default:
				return ".m4a"
			}
		}
	}

	return ""
}

// IsSupportedMIME checks if a MIME type is supported
func IsSupportedMIME(mime string) bool {
	parsed, err := ParseMIME(mime)
	if err != nil {
		return false
	}

	if parsed.IsVideo {
		return true
	}
	if parsed.IsAudio {
		return true
	}
	if parsed.IsSubtitle {
		return true
	}

	// Check against known MIME types
	for _, m := range VideoMIMEs {
		if strings.HasPrefix(mime, m) {
			return true
		}
	}
	for _, m := range AudioMIMEs {
		if strings.HasPrefix(mime, m) {
			return true
		}
	}
	for _, m := range SubtitleMIMEs {
		if strings.HasPrefix(mime, m) {
			return true
		}
	}
	for _, m := range ContainerMIMEs {
		if strings.HasPrefix(mime, m) {
			return true
		}
	}

	return false
}
