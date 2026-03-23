// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package android

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/tenkile/tenkile/internal/probes"
)

// ClientAdapter is implemented by platform-specific probe libraries.
type ClientAdapter interface {
	ClientType() string
	SourceID() string
	ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error)
	TrustScore() float64
}

// AndroidAdapter handles Android device capability reports.
// Android apps use MediaCodec APIs to enumerate hardware decoders
// and report results via the probe API.
type AndroidAdapter struct {
	baseTrust float64
}

// NewAndroidAdapter creates a new Android adapter
func NewAndroidAdapter() *AndroidAdapter {
	return &AndroidAdapter{
		baseTrust: 0.85, // Higher trust than web clients due to native API access
	}
}

// ClientType returns "android"
func (a *AndroidAdapter) ClientType() string {
	return "android"
}

// SourceID returns "android-mediacodec"
func (a *AndroidAdapter) SourceID() string {
	return "android-mediacodec"
}

// TrustScore returns the trust score for Android-reported capabilities
func (a *AndroidAdapter) TrustScore() float64 {
	return a.baseTrust
}

// ExtractCapabilities extracts device capabilities from an Android probe report
func (a *AndroidAdapter) ExtractCapabilities(r *http.Request) (*probes.DeviceCapabilities, error) {
	caps := &probes.DeviceCapabilities{
		Platform: "android",
	}

	// Extract device identity
	caps.DeviceID = r.Header.Get("X-Device-ID")
	if caps.DeviceID == "" {
		caps.DeviceID = r.Header.Get("X-DeviceId")
	}
	caps.UserAgent = r.UserAgent()

	// Extract Android-specific headers
	caps.Manufacturer = r.Header.Get("X-Android-Manufacturer")
	caps.Model = r.Header.Get("X-Android-Model")
	caps.OSVersion = r.Header.Get("X-Android-Version")
	caps.AppVersion = r.Header.Get("X-App-Version")
	caps.DeviceName = r.Header.Get("X-Device-Name")

	// Set platform type
	caps.Identity.Platform.Type = "mobile"
	caps.Identity.Platform.Name = "Android"
	caps.Identity.Platform.OS = "Android"
	caps.Identity.Platform.OSVersion = caps.OSVersion
	caps.Identity.Manufacturer = caps.Manufacturer
	caps.Identity.Model = caps.Model

	// Extract codec information from headers
	a.extractCodecHeaders(r, caps)

	// Try to extract full probe report from request body
	if r.ContentLength > 0 && r.ContentLength < 1<<20 { // Max 1MB
		if err := a.extractProbeBody(r, caps); err != nil {
			// Body extraction failed, but we have header data
		}
	}

	// Apply Android defaults if not already set
	a.applyAndroidDefaults(caps)

	// Calculate trust score based on capability richness
	a.calculateTrustScore(caps)

	return caps, nil
}

// extractCodecHeaders extracts codec support from Android-specific headers
func (a *AndroidAdapter) extractCodecHeaders(r *http.Request, caps *probes.DeviceCapabilities) {
	// Video codecs: X-Android-Video-Codecs: video/avc,video/hevc,video/av01
	if codecs := r.Header.Get("X-Android-Video-Codecs"); codecs != "" {
		caps.VideoCodecs = parseMediaTypes(codecs, "video/")
	}

	// Audio codecs: X-Android-Audio-Codecs: audio/mp4a-latm,audio/ac3,audio/eac3
	if codecs := r.Header.Get("X-Android-Audio-Codecs"); codecs != "" {
		caps.AudioCodecs = parseMediaTypes(codecs, "audio/")
	}

	// HDR types: X-Android-HDR: hdr10,dolby-vision,hdr10plus,hlg
	if hdr := r.Header.Get("X-Android-HDR"); hdr != "" {
		hdrs := strings.Split(strings.ToLower(hdr), ",")
		for _, h := range hdrs {
			h = strings.TrimSpace(h)
			switch h {
			case "hdr10":
				caps.SupportsHDR = true
			case "dolby-vision", "dv":
				caps.SupportsDolbyVision = true
			case "hdr10plus":
				caps.Supports10Bit = true
			case "hlg":
				caps.SupportsHDR = true
			}
		}
	}

	// DRM systems: X-Android-DRM: widevine,playready,clearkey
	if drm := r.Header.Get("X-Android-DRM"); drm != "" {
		caps.DRMSupport = &probes.DRMSupported{
			Supported: true,
			Systems:  strings.Split(strings.ToLower(drm), ","),
		}
	}

	// Max resolution: X-Android-Resolution: 3840x2160
	if res := r.Header.Get("X-Android-Resolution"); res != "" {
		parts := strings.Split(res, "x")
		if len(parts) == 2 {
			if w, _ := strconv.Atoi(strings.TrimSpace(parts[0])); w > 0 {
				caps.MaxWidth = w
			}
			if h, _ := strconv.Atoi(strings.TrimSpace(parts[1])); h > 0 {
				caps.MaxHeight = h
			}
		}
	}

	// Max bitrate: X-Android-Max-Bitrate: 100000000
	if bitrate := r.Header.Get("X-Android-Max-Bitrate"); bitrate != "" {
		if br, err := strconv.ParseInt(bitrate, 10, 64); err == nil {
			caps.MaxBitrate = br
		}
	}

	// Container formats: X-Android-Containers: video/mp4,video/webm,video/x-matroska
	if containers := r.Header.Get("X-Android-Containers"); containers != "" {
		rawContainers := parseMediaTypes(containers, "video/")
		for _, c := range rawContainers {
			if mapped := MapAndroidContainer(c); mapped != "" {
				if !contains(caps.ContainerFormats, mapped) {
					caps.ContainerFormats = append(caps.ContainerFormats, mapped)
				}
			} else {
				// Keep original if not mapped
				if !contains(caps.ContainerFormats, c) {
					caps.ContainerFormats = append(caps.ContainerFormats, c)
				}
			}
		}
	}

	// Secure decoder support: X-Android-Secure-Decoder: true
	if secure := r.Header.Get("X-Android-Secure-Decoder"); secure != "" {
		// Indicates hardware DRM (required for premium content)
	}

	// Vendor codec extensions: X-Android-Vendor-Codecs: ohos.av1,etc
	if vendor := r.Header.Get("X-Android-Vendor-Codecs"); vendor != "" {
		// Additional vendor-specific codec support
	}

	// SoC info: X-Android-SoC: qcom,samsung,mtk
	if soc := r.Header.Get("X-Android-SoC"); soc != "" {
		// System-on-chip vendor for capability inference
	}
}

// extractProbeBody extracts full probe report from JSON body
func (a *AndroidAdapter) extractProbeBody(r *http.Request, caps *probes.DeviceCapabilities) error {
	if r.Body == nil {
		return nil
	}

	var report AndroidProbeReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		return err
	}

	// Map Android codecs to Tenkile codecs
	for _, codec := range report.VideoCodecs {
		if tenkileCodec := MapAndroidCodec(codec.Name, true); tenkileCodec != "" {
			if !contains(caps.VideoCodecs, tenkileCodec) {
				caps.VideoCodecs = append(caps.VideoCodecs, tenkileCodec)
			}
		}
	}

	for _, codec := range report.AudioCodecs {
		if tenkileCodec := MapAndroidCodec(codec.Name, false); tenkileCodec != "" {
			if !contains(caps.AudioCodecs, tenkileCodec) {
				caps.AudioCodecs = append(caps.AudioCodecs, tenkileCodec)
			}
		}
	}

	// Extract HDR capabilities
	for _, feature := range report.HDRFeatures {
		switch feature.Type {
		case "hdr10":
			caps.SupportsHDR = true
		case "dolby-vision", "dv":
			caps.SupportsDolbyVision = true
		case "hdr10plus":
			caps.Supports10Bit = true
		case "hlg":
			caps.SupportsHDR = true
		}
	}

	// Extract DRM info
	if len(report.DRMSystems) > 0 {
		var systems []string
		for _, drm := range report.DRMSystems {
			systems = append(systems, strings.ToLower(drm.Name))
		}
		caps.DRMSupport = &probes.DRMSupported{
			Supported: true,
			Systems:   systems,
		}
	}

	// Extract max resolution
	if report.MaxResolution.Width > 0 {
		caps.MaxWidth = report.MaxResolution.Width
	}
	if report.MaxResolution.Height > 0 {
		caps.MaxHeight = report.MaxResolution.Height
	}

	// Extract bitrate
	if report.MaxBitrate > 0 {
		caps.MaxBitrate = report.MaxBitrate
	}

	// Extract container formats
	for _, container := range report.SupportedContainers {
		if mapped := MapAndroidContainer(container); mapped != "" {
			if !contains(caps.ContainerFormats, mapped) {
				caps.ContainerFormats = append(caps.ContainerFormats, mapped)
			}
		}
	}

	return nil
}

// applyAndroidDefaults sets default capabilities for Android
func (a *AndroidAdapter) applyAndroidDefaults(caps *probes.DeviceCapabilities) {
	// Set default codecs if not specified
	if len(caps.VideoCodecs) == 0 {
		// Most Android devices support at least H.264
		caps.VideoCodecs = []string{"h264"}
	}

	if len(caps.AudioCodecs) == 0 {
		caps.AudioCodecs = []string{"aac", "ac3", "eac3"}
	}

	// Set default containers if not specified
	if len(caps.ContainerFormats) == 0 {
		caps.ContainerFormats = []string{"mp4", "mkv", "webm"}
	}

	// Set default subtitle formats
	if len(caps.SubtitleFormats) == 0 {
		caps.SubtitleFormats = []string{"srt", "vtt", "ssa", "ass"}
	}

	// Set default resolution if not specified
	if caps.MaxWidth == 0 || caps.MaxHeight == 0 {
		caps.MaxWidth = 1920
		caps.MaxHeight = 1080
	}

	// Set default bitrate if not specified
	if caps.MaxBitrate == 0 {
		caps.MaxBitrate = 50_000_000 // 50 Mbps
	}
}

// calculateTrustScore adjusts trust based on Android capability report
func (a *AndroidAdapter) calculateTrustScore(caps *probes.DeviceCapabilities) {
	trust := a.baseTrust

	// Higher trust for full MediaCodec enumeration
	if len(caps.VideoCodecs) >= 3 {
		trust += 0.05
	}

	// Higher trust for AV1 support (requires recent hardware)
	if contains(caps.VideoCodecs, "av1") {
		trust += 0.05
	}

	// Higher trust for Dolby Vision support
	if caps.SupportsDolbyVision {
		trust += 0.05
	}

	// Higher trust for Widevine (secure decoder)
	if caps.DRMSupport != nil && contains(caps.DRMSupport.Systems, "widevine") {
		trust += 0.05
	}

	// Higher trust for 4K support
	if caps.MaxWidth >= 3840 {
		trust += 0.02
	}

	// Cap at 0.98 for native-reported capabilities
	if trust > 0.98 {
		trust = 0.98
	}

	caps.TrustScore = trust
	caps.TrustLevel = probes.TrustLevelFromScore(trust)
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

// parseMediaTypes parses Android MIME types and extracts codec names
func parseMediaTypes(input, prefix string) []string {
	var codecs []string
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.ToLower(part)

		// Extract codec name from MIME type (e.g., "video/avc" -> "avc")
		if strings.HasPrefix(part, prefix) {
			codec := strings.TrimPrefix(part, prefix)
			codec = strings.TrimSpace(codec)
			if codec != "" {
				codecs = append(codecs, codec)
			}
		} else {
			// Already just the codec name
			codecs = append(codecs, part)
		}
	}

	// Map extracted codecs to Tenkile names
	var mapped []string
	for _, codec := range codecs {
		var tenkileCodec string
		if prefix == "video/" {
			tenkileCodec = MapAndroidCodec(codec, true)
		} else {
			tenkileCodec = MapAndroidCodec(codec, false)
		}
		if tenkileCodec != "" && !contains(mapped, tenkileCodec) {
			mapped = append(mapped, tenkileCodec)
		} else if tenkileCodec == "" && !contains(mapped, codec) {
			// Keep original if not mapped
			mapped = append(mapped, codec)
		}
	}

	return mapped
}

// AndroidProbeReport represents the JSON structure sent by Android apps
type AndroidProbeReport struct {
	DeviceID            string               `json:"device_id"`
	Manufacturer        string               `json:"manufacturer"`
	Model               string               `json:"model"`
	AndroidVersion      string               `json:"android_version"`
	AppVersion          string               `json:"app_version"`
	SDKVersion          int                  `json:"sdk_version"`
	VideoCodecs         []AndroidCodec       `json:"video_codecs"`
	AudioCodecs         []AndroidCodec       `json:"audio_codecs"`
	HDRFeatures         []AndroidHDRFeature  `json:"hdr_features"`
	DRMSystems          []AndroidDRMSystem   `json:"drm_systems"`
	MaxResolution       AndroidResolution    `json:"max_resolution"`
	MaxBitrate          int64                `json:"max_bitrate"`
	SupportedContainers []string             `json:"supported_containers"`
	SecureDecoder       bool                 `json:"secure_decoder"`
	VendorCodecs        []string             `json:"vendor_codecs"`
}

// AndroidCodec represents a codec reported by Android MediaCodec
type AndroidCodec struct {
	Name     string `json:"name"`      // e.g., "OMX.qcom.video.encoder.avc"
	Type     string `json:"type"`      // "video/avc" or "audio/mp4a-latm"
	Hardware bool   `json:"hardware"`   // Is this a hardware codec?
	Secure   bool   `json:"secure"`     // Supports secure playback
}

// AndroidHDRFeature represents HDR capability
type AndroidHDRFeature struct {
	Type string `json:"type"` // "hdr10", "dolby-vision", "hdr10plus", "hlg"
}

// AndroidDRMSystem represents DRM capability
type AndroidDRMSystem struct {
	Name  string `json:"name"`  // "widevine", "playready", "clearkey"
	Level string `json:"level"` // Security level (for Widevine)
}

// AndroidResolution represents resolution
type AndroidResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Ensure AndroidAdapter implements ClientAdapter
var _ ClientAdapter = (*AndroidAdapter)(nil)
