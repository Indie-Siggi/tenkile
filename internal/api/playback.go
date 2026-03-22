// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tenkile/tenkile/internal/probes"
	"github.com/tenkile/tenkile/pkg/codec"
)

// PlaybackHandlers holds playback-related API handlers
type PlaybackHandlers struct {
	validator *probes.Validator
	cache     *probes.CapabilityCache
	curatedDB *probes.CuratedDatabase
}

// NewPlaybackHandlers creates new playback handlers
func NewPlaybackHandlers(
	validator *probes.Validator,
	cache *probes.CapabilityCache,
	curatedDB *probes.CuratedDatabase,
) *PlaybackHandlers {
	return &PlaybackHandlers{
		validator: validator,
		cache:     cache,
		curatedDB: curatedDB,
	}
}

// PlaybackDecisionRequest represents a playback decision request
type PlaybackDecisionRequest struct {
	DeviceID       string             `json:"device_id"`
	DeviceHash     string             `json:"device_hash,omitempty"`
	VideoCodec     string             `json:"video_codec"`
	AudioCodec     string             `json:"audio_codec,omitempty"`
	Container      string             `json:"container,omitempty"`
	Resolution     *ResolutionInfo    `json:"resolution,omitempty"`
	Bitrate        int64              `json:"bitrate,omitempty"`
	HasHDR         bool               `json:"has_hdr,omitempty"`
	HasDolbyVision bool               `json:"has_dolby_vision,omitempty"`
	HasDolbyAtmos  bool               `json:"has_dolby_atmos,omitempty"`
	HasDTS         bool               `json:"has_dts,omitempty"`
	SubtitleTracks []SubtitleTrack    `json:"subtitle_tracks,omitempty"`
	StreamInfo     *StreamInformation `json:"stream_info,omitempty"`
}

// Validate validates the playback decision request
func (r *PlaybackDecisionRequest) Validate() error {
	if r.VideoCodec == "" {
		return fmt.Errorf("video_codec is required")
	}
	if r.Bitrate < 0 {
		return fmt.Errorf("bitrate must be non-negative")
	}
	if r.Bitrate > 100000000 {
		return fmt.Errorf("bitrate exceeds maximum allowed value (100 Mbps)")
	}
	if r.Resolution != nil {
		if err := r.Resolution.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ResolutionInfo represents video resolution information
type ResolutionInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Validate validates the resolution info
func (r *ResolutionInfo) Validate() error {
	if r.Width <= 0 || r.Width > 16384 {
		return fmt.Errorf("invalid width: must be between 1 and 16384")
	}
	if r.Height <= 0 || r.Height > 16384 {
		return fmt.Errorf("invalid height: must be between 1 and 16384")
	}
	return nil
}

// SubtitleTrack represents a subtitle track
type SubtitleTrack struct {
	Format   string `json:"format"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Forced   bool   `json:"forced,omitempty"`
	External bool   `json:"external,omitempty"`
}

// StreamInformation represents complete stream information
type StreamInformation struct {
	VideoStreams    []VideoStream    `json:"video_streams"`
	AudioStreams    []AudioStream    `json:"audio_streams"`
	SubtitleStreams []SubtitleStream `json:"subtitle_streams,omitempty"`
	Duration       float64          `json:"duration,omitempty"`
	Size           int64            `json:"size,omitempty"`
}

// VideoStream represents a video stream
type VideoStream struct {
	Codec      string  `json:"codec"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Bitrate    int64   `json:"bitrate,omitempty"`
	Framerate  float64 `json:"framerate,omitempty"`
	Profile    string  `json:"profile,omitempty"`
	Level      float64 `json:"level,omitempty"`
	HasHDR     bool    `json:"has_hdr,omitempty"`
	ColorSpace string  `json:"color_space,omitempty"`
	BitDepth   int     `json:"bit_depth,omitempty"`
}

// AudioStream represents an audio stream
type AudioStream struct {
	Codec      string   `json:"codec"`
	Channels   int      `json:"channels"`
	Bitrate    int64    `json:"bitrate,omitempty"`
	SampleRate int      `json:"sample_rate,omitempty"`
	Languages  []string `json:"languages,omitempty"`
	IsDefault  bool     `json:"is_default,omitempty"`
	IsForced   bool     `json:"is_forced,omitempty"`
}

// SubtitleStream represents a subtitle stream
type SubtitleStream struct {
	Format    string   `json:"format"`
	Codec     string   `json:"codec"`
	Languages []string `json:"languages,omitempty"`
	IsDefault bool     `json:"is_default,omitempty"`
}

// PlaybackDecisionResponse represents a playback decision response
type PlaybackDecisionResponse struct {
	Decision           string           `json:"decision"` // "direct_play", "transcode", "remux"
	Reason             string           `json:"reason"`
	RecommendedProfile string           `json:"recommended_profile,omitempty"`
	TranscodeOptions   *TranscodeOptions `json:"transcode_options,omitempty"`
	Warnings           []string         `json:"warnings,omitempty"`
}

// TranscodeOptions represents transcoding options
type TranscodeOptions struct {
	TargetVideoCodec string `json:"target_video_codec"`
	TargetAudioCodec string `json:"target_audio_codec"`
	TargetResolution string `json:"target_resolution,omitempty"`
	TargetBitrate    int64  `json:"target_bitrate,omitempty"`
	RemoveHDR        bool   `json:"remove_hdr,omitempty"`
	SubtitleMethod   string `json:"subtitle_method,omitempty"` // "burn", "embed", "none"
}

// PlaybackFeedbackRequest represents playback feedback
type PlaybackFeedbackRequest struct {
	DeviceID      string             `json:"device_id"`
	Decision      string             `json:"decision"`
	ActualOutcome string             `json:"actual_outcome"` // "success", "failure", "interruption"
	ErrorCode     string             `json:"error_code,omitempty"`
	ErrorMessage  string             `json:"error_message,omitempty"`
	Duration      float64            `json:"duration,omitempty"`
	StoppedAt     float64            `json:"stopped_at,omitempty"`
	StreamInfo    *StreamInformation `json:"stream_info,omitempty"`
	Timestamp     time.Time          `json:"timestamp"`
}

// handlePlaybackDecision handles playback decision requests
func (h *PlaybackHandlers) handlePlaybackDecision(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req PlaybackDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if err := req.Validate(); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	select {
	case <-ctx.Done():
		RespondJSON(w, http.StatusRequestTimeout, ErrorResponse{
			Error:   "timeout",
			Message: "Request cancelled",
		})
		return
	default:
	}

	// Get or create device capabilities
	var caps *probes.DeviceCapabilities
	if req.DeviceID != "" {
		if cached, found := h.cache.Get(req.DeviceID); found {
			caps = cached
		}
	}

	if caps == nil {
		caps = &probes.DeviceCapabilities{
			DeviceID:         req.DeviceID,
			VideoCodecs:      []string{req.VideoCodec},
			AudioCodecs:      []string{req.AudioCodec},
			ContainerFormats: []string{req.Container},
		}
		if req.Resolution != nil {
			caps.MaxWidth = req.Resolution.Width
			caps.MaxHeight = req.Resolution.Height
		}
		caps.SupportsHDR = req.HasHDR
		caps.SupportsDolbyVision = req.HasDolbyVision
		caps.SupportsDolbyAtmos = req.HasDolbyAtmos
		caps.SupportsDTS = req.HasDTS
	}

	decision, reason, transcodeOptions := h.makeDecision(caps, &req)

	resp := PlaybackDecisionResponse{
		Decision:           decision,
		Reason:             reason,
		RecommendedProfile: determineProfileName(caps),
		TranscodeOptions:   transcodeOptions,
	}

	RespondJSON(w, http.StatusOK, resp)
}

// makeDecision determines the playback decision
func (h *PlaybackHandlers) makeDecision(
	caps *probes.DeviceCapabilities,
	req *PlaybackDecisionRequest,
) (string, string, *TranscodeOptions) {
	if caps == nil {
		return "transcode", "Device capabilities unknown, transcoding recommended", &TranscodeOptions{
			TargetVideoCodec: "h264",
			TargetAudioCodec: "aac",
		}
	}

	// Check video codec compatibility
	videoCompatible := false
	if len(caps.VideoCodecs) == 0 {
		videoCompatible = true
	} else {
		for _, vc := range caps.VideoCodecs {
			if codec.Equal(vc, req.VideoCodec) {
				videoCompatible = true
				break
			}
		}
	}

	// Check audio codec compatibility
	audioCompatible := true
	if req.AudioCodec != "" {
		audioCompatible = false
		for _, ac := range caps.AudioCodecs {
			if codec.Equal(ac, req.AudioCodec) {
				audioCompatible = true
				break
			}
		}
	}

	// Check resolution compatibility
	resolutionCompatible := true
	if req.Resolution != nil && caps.MaxWidth > 0 && caps.MaxHeight > 0 {
		if req.Resolution.Width > caps.MaxWidth || req.Resolution.Height > caps.MaxHeight {
			resolutionCompatible = false
		}
	}

	// Check HDR compatibility
	hdrCompatible := true
	if req.HasHDR && !caps.SupportsHDR {
		hdrCompatible = false
	}
	if req.HasDolbyVision && !caps.SupportsDolbyVision {
		hdrCompatible = false
	}

	// Check bitrate compatibility
	bitrateCompatible := true
	if req.Bitrate > 0 && caps.MaxBitrate > 0 && req.Bitrate > caps.MaxBitrate {
		bitrateCompatible = false
	}

	// Direct play if everything is compatible
	if videoCompatible && audioCompatible && resolutionCompatible && hdrCompatible && bitrateCompatible {
		return "direct_play", "All stream parameters compatible with device", nil
	}

	// Build transcode options
	transcodeOptions := &TranscodeOptions{
		TargetVideoCodec: "h264",
		TargetAudioCodec: "aac",
		SubtitleMethod:   "embed",
	}

	if !videoCompatible {
		for _, target := range []string{"h264", "hevc", "vp9", "av1"} {
			for _, vc := range caps.VideoCodecs {
				if codec.Equal(vc, target) {
					transcodeOptions.TargetVideoCodec = target
					break
				}
			}
		}
	}

	if !audioCompatible {
		for _, target := range []string{"aac", "mp3", "ac3", "opus"} {
			for _, ac := range caps.AudioCodecs {
				if codec.Equal(ac, target) {
					transcodeOptions.TargetAudioCodec = target
					break
				}
			}
		}
	}

	if !hdrCompatible {
		transcodeOptions.RemoveHDR = true
	}

	if !bitrateCompatible && caps.MaxBitrate > 0 {
		transcodeOptions.TargetBitrate = caps.MaxBitrate / 2
	}

	reason := "Stream requires transcoding"
	if !videoCompatible {
		reason = "Video codec not supported"
	} else if !audioCompatible {
		reason = "Audio codec not supported"
	} else if !resolutionCompatible {
		reason = "Resolution exceeds device capability"
	} else if !hdrCompatible {
		reason = "HDR format not supported"
	} else if !bitrateCompatible {
		reason = "Bitrate exceeds device capability"
	}

	return "transcode", reason, transcodeOptions
}

// handlePlaybackFeedback handles playback feedback
func (h *PlaybackHandlers) handlePlaybackFeedback(w http.ResponseWriter, r *http.Request) {
	var req PlaybackFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	if req.DeviceID == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_device_id",
			Message: "Device ID is required",
		})
		return
	}

	if req.ActualOutcome == "" {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "missing_outcome",
			Message: "Actual outcome is required",
		})
		return
	}

	slog.Info("Playback feedback received",
		"device_id", req.DeviceID,
		"decision", req.Decision,
		"outcome", req.ActualOutcome,
		"error_code", req.ErrorCode,
	)

	RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "received",
		"message": "Feedback recorded",
	})
}

// handleTranscodeRecommendation handles transcode recommendation requests
func (h *PlaybackHandlers) handleTranscodeRecommendation(w http.ResponseWriter, r *http.Request) {
	var req PlaybackDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	var caps *probes.DeviceCapabilities
	if req.DeviceID != "" {
		if cached, found := h.cache.Get(req.DeviceID); found {
			caps = cached
		}
	}

	if caps == nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "unknown_device",
			Message: "Device capabilities not found",
		})
		return
	}

	options := &TranscodeOptions{
		TargetVideoCodec: "h264",
		TargetAudioCodec: "aac",
		SubtitleMethod:   "embed",
	}

	for _, target := range []string{"h264", "hevc", "vp9", "av1"} {
		for _, vc := range caps.VideoCodecs {
			if codec.Equal(vc, target) {
				options.TargetVideoCodec = target
				break
			}
		}
	}

	for _, target := range []string{"aac", "mp3", "ac3", "eac3", "opus"} {
		for _, ac := range caps.AudioCodecs {
			if codec.Equal(ac, target) {
				options.TargetAudioCodec = target
				break
			}
		}
	}

	if caps.MaxBitrate > 0 {
		options.TargetBitrate = caps.MaxBitrate / 2
	}
	if caps.MaxWidth > 0 && caps.MaxHeight > 0 {
		options.TargetResolution = caps.FormatResolution()
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"profile":           determineProfileName(caps),
		"transcode_options": options,
	})
}

// handleGetProfiles handles quality profile requests
func (h *PlaybackHandlers) handleGetProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := []map[string]interface{}{
		{"name": "ultra_hd_hdr", "display_name": "Ultra HD HDR", "max_resolution": "3840x2160", "codecs": []string{"hevc", "h264"}, "hdr": true, "description": "4K resolution with HDR support"},
		{"name": "ultra_hd", "display_name": "Ultra HD", "max_resolution": "3840x2160", "codecs": []string{"hevc", "h264"}, "hdr": false, "description": "4K resolution without HDR"},
		{"name": "full_hd_hdr", "display_name": "Full HD HDR", "max_resolution": "1920x1080", "codecs": []string{"h264", "hevc"}, "hdr": true, "description": "1080p resolution with HDR support"},
		{"name": "full_hd", "display_name": "Full HD", "max_resolution": "1920x1080", "codecs": []string{"h264"}, "hdr": false, "description": "1080p resolution"},
		{"name": "hd", "display_name": "HD", "max_resolution": "1280x720", "codecs": []string{"h264"}, "hdr": false, "description": "720p resolution"},
		{"name": "sd", "display_name": "SD", "max_resolution": "854x480", "codecs": []string{"h264"}, "hdr": false, "description": "480p resolution"},
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"profiles": profiles,
		"count":    len(profiles),
	})
}

// handleValidateStream handles stream validation requests
func (h *PlaybackHandlers) handleValidateStream(w http.ResponseWriter, r *http.Request) {
	var req PlaybackDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to parse request body",
		})
		return
	}

	var caps *probes.DeviceCapabilities
	if req.DeviceID != "" {
		if cached, found := h.cache.Get(req.DeviceID); found {
			caps = cached
		}
	}

	if caps == nil {
		RespondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "device_not_found",
			Message: "Device capabilities not found",
		})
		return
	}

	videoValid := false
	for _, vc := range caps.VideoCodecs {
		if codec.Equal(vc, req.VideoCodec) {
			videoValid = true
			break
		}
	}

	audioValid := true
	if req.AudioCodec != "" {
		audioValid = false
		for _, ac := range caps.AudioCodecs {
			if codec.Equal(ac, req.AudioCodec) {
				audioValid = true
				break
			}
		}
	}

	resolutionValid := true
	if req.Resolution != nil && caps.MaxWidth > 0 && caps.MaxHeight > 0 {
		if req.Resolution.Width > caps.MaxWidth || req.Resolution.Height > caps.MaxHeight {
			resolutionValid = false
		}
	}

	hdrValid := true
	if req.HasHDR && !caps.SupportsHDR {
		hdrValid = false
	}

	var warnings []string
	if !videoValid {
		warnings = append(warnings, "Video codec not supported: "+req.VideoCodec)
	}
	if !audioValid && req.AudioCodec != "" {
		warnings = append(warnings, "Audio codec not supported: "+req.AudioCodec)
	}
	if !resolutionValid {
		warnings = append(warnings, "Resolution exceeds device capability")
	}
	if !hdrValid {
		warnings = append(warnings, "HDR format not supported")
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"valid": videoValid && audioValid && resolutionValid && hdrValid,
		"checks": map[string]bool{
			"video_codec": videoValid,
			"audio_codec": audioValid,
			"resolution":  resolutionValid,
			"hdr":         hdrValid,
		},
		"warnings": warnings,
	})
}
