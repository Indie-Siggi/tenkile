// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package codec

import "strings"

// CodecType represents the type of codec
type CodecType int

const (
	CodecTypeVideo CodecType = iota
	CodecTypeAudio
	CodecTypeSubtitle
)

// String returns the string representation of CodecType
func (c CodecType) String() string {
	switch c {
	case CodecTypeVideo:
		return "video"
	case CodecTypeAudio:
		return "audio"
	case CodecTypeSubtitle:
		return "subtitle"
	default:
		return "unknown"
	}
}

// ProfileType represents a quality profile type
type ProfileType int

const (
	ProfileTypeOriginal ProfileType = iota
	ProfileTypeUltraHD
	ProfileTypeFullHD
	ProfileTypeHD
	ProfileTypeSD
	ProfileTypeMobile
)

// String returns the string representation of ProfileType
func (p ProfileType) String() string {
	switch p {
	case ProfileTypeOriginal:
		return "original"
	case ProfileTypeUltraHD:
		return "ultra_hd"
	case ProfileTypeFullHD:
		return "full_hd"
	case ProfileTypeHD:
		return "hd"
	case ProfileTypeSD:
		return "sd"
	case ProfileTypeMobile:
		return "mobile"
	default:
		return "unknown"
	}
}

// Codec represents a video, audio, or subtitle codec
type Codec struct {
	Name        string     `json:"name"`
	Type        CodecType  `json:"type"`
	Description string     `json:"description"`
	Container   []string   `json:"containers"`
	Profiles    []string   `json:"profiles"`
	Bandwidth   int64      `json:"bandwidth"` // Typical bandwidth in bits per second
	MaxWidth    int        `json:"max_width"`
	MaxHeight   int        `json:"max_height"`
	MaxFramerate int       `json:"max_framerate"`
	BitDepth    int        `json:"bit_depth"`
	HDRSupport  bool       `json:"hdr_support"`
	License     string     `json:"license"` // "open", "patent", "proprietary"
}

// Container represents a media container format
type Container struct {
	Name          string   `json:"name"`
	Extensions    []string `json:"extensions"`
	MimeTypes     []string `json:"mime_types"`
	SupportedCodecs []string `json:"supported_codecs"`
	Limitations   []string `json:"limitations,omitempty"`
}

// Profiles defines quality profiles with bitrate/resolution constraints
type Profiles struct {
	Name        string `json:"name"`
	MaxWidth    int    `json:"max_width"`
	MaxHeight   int    `json:"max_height"`
	MaxBitrate  int64  `json:"max_bitrate"`
	AudioBitrate int64 `json:"audio_bitrate"`
}

// Equal checks if two codec names refer to the same codec (case-insensitive).
func Equal(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// Codec database - map of codec name to codec definition
var codecs = map[string]*Codec{
	// Video codecs
	"h264": {
		Name:        "H.264",
		Type:        CodecTypeVideo,
		Description: "Advanced Video Coding (AVC)",
		Container:   []string{"mp4", "mkv", "ts", "mov", "webm"},
		Profiles:    []string{"baseline", "main", "high"},
		Bandwidth:   8000000,
		MaxWidth:    3840,
		MaxHeight:   2160,
		MaxFramerate: 60,
		BitDepth:    8,
		HDRSupport:  false,
		License:     "patent",
	},
	"hevc": {
		Name:        "HEVC",
		Type:        CodecTypeVideo,
		Description: "High Efficiency Video Coding (H.265)",
		Container:   []string{"mp4", "mkv", "ts", "mov"},
		Profiles:    []string{"main", "main10"},
		Bandwidth:   15000000,
		MaxWidth:    7680,
		MaxHeight:   4320,
		MaxFramerate: 120,
		BitDepth:    10,
		HDRSupport:  true,
		License:     "patent",
	},
	"vp9": {
		Name:        "VP9",
		Type:        CodecTypeVideo,
		Description: "VP9 video codec",
		Container:   []string{"webm", "mkv", "mp4"},
		Profiles:    []string{"0", "2"},
		Bandwidth:   12000000,
		MaxWidth:    7680,
		MaxHeight:   4320,
		MaxFramerate: 60,
		BitDepth:    10,
		HDRSupport:  true,
		License:     "open",
	},
	"av1": {
		Name:        "AV1",
		Type:        CodecTypeVideo,
		Description: "AOMedia Video 1",
		Container:   []string{"webm", "mkv", "mp4"},
		Profiles:    []string{"main", "high"},
		Bandwidth:   20000000,
		MaxWidth:    7680,
		MaxHeight:   4320,
		MaxFramerate: 120,
		BitDepth:    10,
		HDRSupport:  true,
		License:     "open",
	},
	"mpeg2": {
		Name:        "MPEG-2",
		Type:        CodecTypeVideo,
		Description: "MPEG-2 Part 2",
		Container:   []string{"ts", "mpg", "mpeg"},
		Profiles:    []string{"main"},
		Bandwidth:   15000000,
		MaxWidth:    1920,
		MaxHeight:   1080,
		MaxFramerate: 60,
		BitDepth:    8,
		HDRSupport:  false,
		License:     "patent",
	},
	"mpeg4": {
		Name:        "MPEG-4 Part 2",
		Type:        CodecTypeVideo,
		Description: "MPEG-4 Part 2 Visual",
		Container:   []string{"avi", "mp4", "mkv"},
		Profiles:    []string{"simple", "advanced"},
		Bandwidth:   4000000,
		MaxWidth:    1920,
		MaxHeight:   1080,
		MaxFramerate: 30,
		BitDepth:    8,
		HDRSupport:  false,
		License:     "patent",
	},
	"vc1": {
		Name:        "VC-1",
		Type:        CodecTypeVideo,
		Description: "SMPTE 421M (Windows Media Video 9)",
		Container:   []string{"wmv", "asf", "mp4"},
		Profiles:    []string{"simple", "main", "advanced"},
		Bandwidth:   10000000,
		MaxWidth:    1920,
		MaxHeight:   1080,
		MaxFramerate: 30,
		BitDepth:    8,
		HDRSupport:  false,
		License:     "patent",
	},
	"vp8": {
		Name:        "VP8",
		Type:        CodecTypeVideo,
		Description: "VP8 video codec",
		Container:   []string{"webm", "mkv"},
		Profiles:    []string{"main"},
		Bandwidth:   6000000,
		MaxWidth:    1920,
		MaxHeight:   1080,
		MaxFramerate: 60,
		BitDepth:    8,
		HDRSupport:  false,
		License:     "open",
	},

	// Audio codecs
	"aac": {
		Name:        "AAC",
		Type:        CodecTypeAudio,
		Description: "Advanced Audio Coding",
		Container:   []string{"mp4", "m4a", "ts", "mkv"},
		Profiles:    []string{"lc", "he", "he_v2"},
		Bandwidth:   320000,
		License:     "patent",
	},
	"mp3": {
		Name:        "MP3",
		Type:        CodecTypeAudio,
		Description: "MPEG-1 Audio Layer III",
		Container:   []string{"mp3", "mp4", "mkv"},
		Bandwidth:   320000,
		License:     "patent",
	},
	"flac": {
		Name:        "FLAC",
		Type:        CodecTypeAudio,
		Description: "Free Lossless Audio Codec",
		Container:   []string{"flac", "mkv", "ogg"},
		Bandwidth:   1411200,
		License:     "open",
	},
	"alac": {
		Name:        "ALAC",
		Type:        CodecTypeAudio,
		Description: "Apple Lossless Audio Codec",
		Container:   []string{"m4a", "mkv"},
		Bandwidth:   1411200,
		License:     "open",
	},
	"opus": {
		Name:        "Opus",
		Type:        CodecTypeAudio,
		Description: "Opus interactive audio codec",
		Container:   []string{"ogg", "opus", "webm", "mkv"},
		Bandwidth:   510000,
		License:     "open",
	},
	"ac3": {
		Name:        "AC-3",
		Type:        CodecTypeAudio,
		Description: "Dolby Digital",
		Container:   []string{"mp4", "ts", "mkv", "avi"},
		Bandwidth:   640000,
		License:     "patent",
	},
	"eac3": {
		Name:        "E-AC-3",
		Type:        CodecTypeAudio,
		Description: "Dolby Digital Plus",
		Container:   []string{"mp4", "ts", "mkv"},
		Bandwidth:   1536000,
		License:     "patent",
	},
	"dts": {
		Name:        "DTS",
		Type:        CodecTypeAudio,
		Description: "DTS Coherent Acoustics",
		Container:   []string{"mkv", "ts", "mp4"},
		Bandwidth:   1536000,
		License:     "patent",
	},
	"truehd": {
		Name:        "TrueHD",
		Type:        CodecTypeAudio,
		Description: "Dolby TrueHD",
		Container:   []string{"mkv", "mp4", "ts"},
		Bandwidth:   18000000,
		License:     "patent",
	},
	"aac_low": {
		Name:        "AAC-LC",
		Type:        CodecTypeAudio,
		Description: "AAC Low Complexity",
		Container:   []string{"mp4", "m4a", "ts"},
		Bandwidth:   256000,
		License:     "patent",
	},
	"aac_he": {
		Name:        "AAC-HE",
		Type:        CodecTypeAudio,
		Description: "AAC High Efficiency",
		Container:   []string{"mp4", "m4a", "ts"},
		Bandwidth:   128000,
		License:     "patent",
	},
	"aac_he_v2": {
		Name:        "AAC-HEv2",
		Type:        CodecTypeAudio,
		Description: "AAC High Efficiency v2",
		Container:   []string{"mp4", "m4a", "ts"},
		Bandwidth:   96000,
		License:     "patent",
	},
}

// containers database
var containers = map[string]*Container{
	"mp4": {
		Name:          "MPEG-4 Part 14",
		Extensions:    []string{".mp4", ".m4a", ".m4v"},
		MimeTypes:     []string{"video/mp4", "audio/mp4"},
		SupportedCodecs: []string{"h264", "hevc", "vp9", "aac", "mp3", "flac", "alac"},
	},
	"mkv": {
		Name:          "Matroska",
		Extensions:    []string{".mkv", ".mka", ".mks"},
		MimeTypes:     []string{"video/x-matroska", "audio/x-matroska"},
		SupportedCodecs: []string{"h264", "hevc", "vp9", "av1", "mpeg2", "aac", "mp3", "flac", "alac", "opus", "ac3", "eac3", "dts", "truehd"},
	},
	"ts": {
		Name:          "MPEG Transport Stream",
		Extensions:    []string{".ts", ".m2ts", ".mts"},
		MimeTypes:     []string{"video/mp2t"},
		SupportedCodecs: []string{"h264", "hevc", "mpeg2", "aac", "mp3", "ac3", "eac3", "dts"},
	},
	"webm": {
		Name:          "WebM",
		Extensions:    []string{".webm"},
		MimeTypes:     []string{"video/webm", "audio/webm"},
		SupportedCodecs: []string{"vp8", "vp9", "av1", "opus", "aac", "flac"},
		Limitations:   []string{"Limited browser support for some codecs"},
	},
	"mov": {
		Name:          "QuickTime File Format",
		Extensions:    []string{".mov", ".qt"},
		MimeTypes:     []string{"video/quicktime"},
		SupportedCodecs: []string{"h264", "hevc", "aac", "mp3"},
	},
	"avi": {
		Name:          "Audio Video Interleave",
		Extensions:    []string{".avi"},
		MimeTypes:     []string{"video/x-msvideo"},
		SupportedCodecs: []string{"h264", "mpeg4", "mp3", "aac"},
		Limitations:   []string{"Limited codec support", "Large file size"},
	},
	"flv": {
		Name:          "Flash Video",
		Extensions:    []string{".flv"},
		MimeTypes:     []string{"video/x-flv"},
		SupportedCodecs: []string{"h264", "vp6", "aac", "mp3"},
		Limitations:   []string{"Legacy format", "Limited codec support"},
	},
}

// GetByName returns a codec by name
func GetByName(name string) (*Codec, bool) {
	codec, ok := codecs[name]
	return codec, ok
}

// GetVideoCodecs returns all video codecs
func GetVideoCodecs() []*Codec {
	var result []*Codec
	for _, c := range codecs {
		if c.Type == CodecTypeVideo {
			result = append(result, c)
		}
	}
	return result
}

// GetAudioCodecs returns all audio codecs
func GetAudioCodecs() []*Codec {
	var result []*Codec
	for _, c := range codecs {
		if c.Type == CodecTypeAudio {
			result = append(result, c)
		}
	}
	return result
}

// GetContainer returns a container by name
func GetContainer(name string) (*Container, bool) {
	container, ok := containers[name]
	return container, ok
}

// GetSupportedContainers returns all containers
func GetSupportedContainers() []*Container {
	var result []*Container
	for _, c := range containers {
		result = append(result, c)
	}
	return result
}

// IsVideoCodec checks if a codec is a video codec
func IsVideoCodec(name string) bool {
	c, ok := codecs[name]
	return ok && c.Type == CodecTypeVideo
}

// IsAudioCodec checks if a codec is an audio codec
func IsAudioCodec(name string) bool {
	c, ok := codecs[name]
	return ok && c.Type == CodecTypeAudio
}

// CanContainerContain checks if a container can hold a codec
func CanContainerContain(container, codec string) bool {
	c, ok := containers[container]
	if !ok {
		return false
	}
	for _, supported := range c.SupportedCodecs {
		if supported == codec {
			return true
		}
	}
	return false
}
